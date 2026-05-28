/**
 * velane bun executor runner
 *
 * Persistent HTTP server that accepts POST /run and POST /run/stream requests,
 * writes snippet code to a temporary file, executes it as a Bun subprocess with
 * the input payload available as SNIPPET_INPUT, captures stdout/stderr, enforces
 * a timeout, and returns a structured result.
 *
 * Expected request body:
 *   { code: string, input: string, timeout_ms: number, max_memory_mb: number,
 *     secret_env_vars?: Record<string,string>, egress_policy?: EgressPolicy }
 *
 * POST /run response body:
 *   { output: string, stderr: string, duration_ms: number, peak_memory_mb: number, exit_code: number, error: string }
 *
 * POST /run/stream response: text/event-stream
 *   data: {"chunk":"...", "done":false}\n\n
 *   ...
 *   data: {"chunk":"...", "done":true}\n\n
 *
 * Snippet convention:
 *   The snippet file must export a default async function `handler` that
 *   accepts the parsed input and returns the output OR an async generator
 *   that yields chunks. The runner wraps execution and prints/streams the result.
 *
 * Example snippet (plain handler):
 *   export default async function handler(input: { name: string }) {
 *     return { greeting: `Hello, ${input.name}!` };
 *   }
 *
 * Example snippet (streaming handler):
 *   export default async function* handler(input: { n: number }) {
 *     for (let i = 0; i < input.n; i++) {
 *       yield { chunk: i };
 *     }
 *   }
 */

import { randomBytes } from "node:crypto";
import { mkdtemp, writeFile, mkdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";

const PORT = 8080;

interface EgressPolicy {
  blocked_cidrs: string[];
  blocked_domains: string[];
}

interface RunRequest {
  code: string;
  input: string;
  timeout_ms: number;
  max_memory_mb: number;
  secret_env_vars?: Record<string, string>;
  libraries?: Record<string, string>;
  egress_policy?: EgressPolicy;
}

interface RunResult {
  output: string;
  stderr: string;
  duration_ms: number;
  peak_memory_mb: number;
  exit_code: number;
  error: string;
}

/**
 * Build the env vars to pass to the subprocess.
 * Merges process.env with any secret_env_vars from the run request.
 */
function buildEnv(req: RunRequest): Record<string, string> {
  const env: Record<string, string> = { ...process.env } as Record<string, string>;
  env["SNIPPET_INPUT"] = req.input;
  for (const [key, val] of Object.entries(req.secret_env_vars ?? {})) {
    env[key] = val;
  }
  return env;
}

/**
 * Write the wrapper harness that imports the snippet and runs handler().
 * The snippet's output is printed as JSON to stdout.
 * If an egress_policy is present, wraps the global fetch to block domains.
 */
function buildHarnessCode(snippetPath: string, input: string, egressPolicy?: EgressPolicy): string {
  // Escape the input and path for safe embedding in JS source.
  const safeInput = JSON.stringify(input);
  const safePath = JSON.stringify(snippetPath);
  const safeEgress = JSON.stringify(egressPolicy ?? null);

  return `
import * as snippetModule from ${safePath};

const handler = typeof snippetModule.default === 'function'
  ? snippetModule.default
  : typeof snippetModule.handler === 'function'
  ? snippetModule.handler
  : null;

if (typeof handler !== 'function') {
  console.error('Snippet must export a default function (export default async function handler) or a named "handler" export');
  process.exit(1);
}

// Egress policy enforcement: wrap fetch to block disallowed domains.
const egressPolicy = ${safeEgress};
if (egressPolicy && egressPolicy.blocked_domains && egressPolicy.blocked_domains.length > 0) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    const url = typeof input === 'string' ? new URL(input) : new URL(input.url);
    const hostname = url.hostname;
    for (const domain of egressPolicy.blocked_domains) {
      if (hostname === domain || hostname.endsWith('.' + domain)) {
        throw new Error('Egress blocked: ' + hostname + ' is in the domain blocklist');
      }
    }
    return originalFetch(input, init);
  };
}

const rawInput = ${safeInput};
let parsedInput;
try {
  parsedInput = JSON.parse(rawInput);
} catch (e) {
  parsedInput = rawInput;
}

(async () => {
  try {
    const result = await handler(parsedInput);
    const output = typeof result === 'string' ? result : JSON.stringify(result);
    process.stdout.write(output + '\\n');
    process.exit(0);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    process.stderr.write('handler error: ' + msg + '\\n');
    process.exit(1);
  }
})();
`.trimStart();
}

/**
 * Write the streaming harness that imports the snippet and yields chunks as
 * SSE-formatted JSON to stdout. Each line is a complete JSON object.
 * If an egress_policy is present, wraps the global fetch to block domains.
 */
function buildStreamHarnessCode(snippetPath: string, input: string, egressPolicy?: EgressPolicy): string {
  const safeInput = JSON.stringify(input);
  const safePath = JSON.stringify(snippetPath);
  const safeEgress = JSON.stringify(egressPolicy ?? null);

  return `
import * as snippetModule from ${safePath};

const handler = typeof snippetModule.default === 'function'
  ? snippetModule.default
  : typeof snippetModule.handler === 'function'
  ? snippetModule.handler
  : null;

if (typeof handler !== 'function') {
  console.error(JSON.stringify({ chunk: '', error: 'Snippet must export a default function (export default async function handler) or a named "handler" export', done: true }));
  process.exit(1);
}

// Egress policy enforcement: wrap fetch to block disallowed domains.
const egressPolicy = ${safeEgress};
if (egressPolicy && egressPolicy.blocked_domains && egressPolicy.blocked_domains.length > 0) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input, init) => {
    const url = typeof input === 'string' ? new URL(input) : new URL(input.url);
    const hostname = url.hostname;
    for (const domain of egressPolicy.blocked_domains) {
      if (hostname === domain || hostname.endsWith('.' + domain)) {
        throw new Error('Egress blocked: ' + hostname + ' is in the domain blocklist');
      }
    }
    return originalFetch(input, init);
  };
}

const rawInput = ${safeInput};
let parsedInput;
try {
  parsedInput = JSON.parse(rawInput);
} catch (e) {
  parsedInput = rawInput;
}

(async () => {
  try {
    const result = await handler(parsedInput);

    // Check if the result is an async generator.
    if (result !== null && typeof result === 'object' && typeof result[Symbol.asyncIterator] === 'function') {
      for await (const item of result) {
        const chunk = typeof item === 'string' ? item : JSON.stringify(item);
        process.stdout.write(JSON.stringify({ chunk, done: false }) + '\\n');
      }
      process.stdout.write(JSON.stringify({ chunk: '', done: true }) + '\\n');
    } else {
      // Plain return value — emit as a single chunk.
      const chunk = typeof result === 'string' ? result : JSON.stringify(result);
      process.stdout.write(JSON.stringify({ chunk, done: true }) + '\\n');
    }
    process.exit(0);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    process.stdout.write(JSON.stringify({ chunk: '', error: msg, done: true }) + '\\n');
    process.exit(1);
  }
})();
`.trimStart();
}

/**
 * Write library source files into workDir/node_modules/{importPath}/index.ts
 * so Bun's native module resolution picks them up automatically.
 * importPath is e.g. "@velane/http-client" or "@myorg/data-utils".
 */
async function writeLibraries(workDir: string, libraries: Record<string, string> | undefined): Promise<void> {
  if (!libraries) return;
  for (const [importPath, code] of Object.entries(libraries)) {
    const libDir = join(workDir, "node_modules", importPath);
    await mkdir(libDir, { recursive: true });
    await writeFile(join(libDir, "index.ts"), code, "utf8");
  }
}

async function runSnippet(req: RunRequest): Promise<RunResult> {
  const id = randomBytes(8).toString("hex");
  const workDir = await mkdtemp(join(tmpdir(), `rune_${id}_`));
  const snippetPath = join(workDir, "snippet.ts");
  const harnessPath = join(workDir, "harness.ts");

  try {
    await writeLibraries(workDir, req.libraries);
    await writeFile(snippetPath, req.code, "utf8");
    const harnessCode = buildHarnessCode(snippetPath, req.input, req.egress_policy);
    await writeFile(harnessPath, harnessCode, "utf8");

    const timeoutMs = req.timeout_ms > 0 ? req.timeout_ms : 30_000;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    const start = Date.now();
    let proc: ReturnType<typeof Bun.spawn> | null = null;

    // Build env: merge process.env + secret_env_vars.
    const env = buildEnv(req);

    try {
      proc = Bun.spawn(["bun", "run", harnessPath], {
        env,
        stdout: "pipe",
        stderr: "pipe",
        signal: controller.signal,
      });

      const stdoutChunks: Uint8Array[] = [];
      const stderrChunks: Uint8Array[] = [];

      // Collect stdout.
      const collectStdout = async () => {
        if (!proc!.stdout) return;
        const reader = proc!.stdout.getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            if (value) stdoutChunks.push(value);
          }
        } catch {
          // Stream ended or process killed.
        }
      };

      // Collect stderr.
      const collectStderr = async () => {
        if (!proc!.stderr) return;
        const reader = proc!.stderr.getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            if (value) stderrChunks.push(value);
          }
        } catch {
          // Stream ended or process killed.
        }
      };

      await Promise.all([
        collectStdout(),
        collectStderr(),
        proc.exited,
      ]);

      clearTimeout(timer);
      const durationMs = Date.now() - start;
      const exitCode = proc.exitCode ?? -1;

      const decoder = new TextDecoder();
      const stdout = decoder.decode(
        Buffer.concat(stdoutChunks.map((c) => Buffer.from(c)))
      ).trim();
      const stderr = decoder.decode(
        Buffer.concat(stderrChunks.map((c) => Buffer.from(c)))
      ).trim();

      return {
        output: stdout,
        stderr,
        duration_ms: durationMs,
        peak_memory_mb: 0, // not available without cgroups
        exit_code: exitCode,
        error: exitCode !== 0 ? "non-zero exit" : "",
      };
    } catch (err: any) {
      clearTimeout(timer);
      const durationMs = Date.now() - start;

      if (err?.name === "AbortError" || controller.signal.aborted) {
        // Kill the process if it's still running.
        try {
          proc?.kill();
        } catch {}
        return {
          output: "",
          stderr: "execution timed out",
          duration_ms: durationMs,
          peak_memory_mb: 0,
          exit_code: -1,
          error: "timeout",
        };
      }

      return {
        output: "",
        stderr: String(err),
        duration_ms: durationMs,
        peak_memory_mb: 0,
        exit_code: -1,
        error: String(err),
      };
    }
  } finally {
    // Clean up temp directory.
    try {
      await rm(workDir, { recursive: true, force: true });
    } catch {}
  }
}

/**
 * Run a snippet in streaming mode. Returns a ReadableStream of SSE-formatted
 * lines. Each line in the stream harness stdout is a JSON object:
 *   { chunk: string, done: boolean, error?: string }
 * This function wraps them as proper SSE events.
 */
async function runSnippetStream(req: RunRequest): Promise<ReadableStream<Uint8Array>> {
  const id = randomBytes(8).toString("hex");
  const workDir = await mkdtemp(join(tmpdir(), `rune_${id}_`));
  const snippetPath = join(workDir, "snippet.ts");
  const harnessPath = join(workDir, "harness.ts");

  const encoder = new TextEncoder();

  // Build env: merge process.env + secret_env_vars.
  const env = buildEnv(req);

  return new ReadableStream<Uint8Array>({
    async start(controller) {
      try {
        await writeLibraries(workDir, req.libraries);
        await writeFile(snippetPath, req.code, "utf8");
        const harnessCode = buildStreamHarnessCode(snippetPath, req.input, req.egress_policy);
        await writeFile(harnessPath, harnessCode, "utf8");

        const timeoutMs = req.timeout_ms > 0 ? req.timeout_ms : 30_000;
        const abortController = new AbortController();
        const timer = setTimeout(() => abortController.abort(), timeoutMs);

        let proc: ReturnType<typeof Bun.spawn> | null = null;

        try {
          proc = Bun.spawn(["bun", "run", harnessPath], {
            env,
            stdout: "pipe",
            stderr: "pipe",
            signal: abortController.signal,
          });

          // Read stdout line by line and emit SSE events.
          const reader = proc.stdout!.getReader();
          const decoder = new TextDecoder();
          let buffer = "";

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });

            let newline: number;
            while ((newline = buffer.indexOf("\n")) !== -1) {
              const line = buffer.slice(0, newline).trim();
              buffer = buffer.slice(newline + 1);

              if (!line) continue;

              // Each line from the harness is a JSON object.
              // Wrap it as an SSE data event.
              controller.enqueue(encoder.encode(`data: ${line}\n\n`));

              try {
                const parsed = JSON.parse(line);
                if (parsed.done) {
                  // We've received the terminal chunk — stop reading.
                  clearTimeout(timer);
                  try { proc?.kill(); } catch {}
                  controller.close();
                  return;
                }
              } catch {
                // Not valid JSON — continue.
              }
            }
          }

          clearTimeout(timer);

          // Process exited without a done=true line — emit a final event.
          const exitCode = proc.exitCode ?? -1;
          const errMsg = exitCode !== 0 ? "non-zero exit" : "";
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify({ chunk: "", done: true, error: errMsg })}\n\n`)
          );
        } catch (err: any) {
          clearTimeout(timer);
          try { proc?.kill(); } catch {}

          const isTimeout = err?.name === "AbortError" || abortController.signal.aborted;
          const errMsg = isTimeout ? "timeout" : String(err);
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify({ chunk: "", done: true, error: errMsg })}\n\n`)
          );
        }
      } catch (err: any) {
        controller.enqueue(
          encoder.encode(`data: ${JSON.stringify({ chunk: "", done: true, error: String(err) })}\n\n`)
        );
      } finally {
        try {
          await rm(workDir, { recursive: true, force: true });
        } catch {}
        try { controller.close(); } catch {}
      }
    },
  });
}

const server = Bun.serve({
  port: PORT,
  async fetch(req: Request) {
    const url = new URL(req.url);

    // Health check.
    if (req.method === "GET" && url.pathname === "/healthz") {
      return new Response(JSON.stringify({ status: "ok" }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    // --- POST /run/stream ---
    if (req.method === "POST" && url.pathname === "/run/stream") {
      let body: RunRequest;
      try {
        body = (await req.json()) as RunRequest;
      } catch {
        return new Response(JSON.stringify({ error: "invalid JSON body" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (!body.code) {
        return new Response(JSON.stringify({ error: "code is required" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }

      const stream = await runSnippetStream(body);
      return new Response(stream, {
        headers: {
          "Content-Type": "text/event-stream",
          "Cache-Control": "no-cache",
          "X-Accel-Buffering": "no",
        },
      });
    }

    // --- POST /run ---
    if (req.method === "POST" && url.pathname === "/run") {
      let body: RunRequest;
      try {
        body = (await req.json()) as RunRequest;
      } catch {
        return new Response(JSON.stringify({ error: "invalid JSON body" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }

      if (!body.code) {
        return new Response(JSON.stringify({ error: "code is required" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        });
      }

      const result = await runSnippet(body);
      return new Response(JSON.stringify(result), {
        headers: { "Content-Type": "application/json" },
      });
    }

    return new Response(JSON.stringify({ error: "not found" }), {
      status: 404,
      headers: { "Content-Type": "application/json" },
    });
  },
});

console.log(`bun executor listening on :${PORT}`);
