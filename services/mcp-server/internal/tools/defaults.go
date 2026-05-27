package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/runeforge/mcp-server/internal/controlplane"
)

func (r *Registry) addDefaults() {
	r.add(Tool{
		Name:        "list_snippets",
		Description: "List snippets available to the authenticated tenant.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handle: func(ctx context.Context, authHeader string, _ map[string]any) (any, error) {
			var out []map[string]any
			if err := r.client.Get(ctx, authHeader, "/v1/snippets", &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "get_snippet",
		Description: "Get a snippet by ID.",
		InputSchema: map[string]any{
			"type": "object",
			"required": []string{
				"snippet_id",
			},
			"properties": map[string]any{
				"snippet_id": map[string]any{"type": "string"},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			snippetID, err := toString(args, "snippet_id", true)
			if err != nil {
				return nil, err
			}
			var out map[string]any
			if err := r.client.Get(ctx, authHeader, "/v1/snippets/"+url.PathEscape(snippetID), &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "create_snippet",
		Description: "Create a snippet.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"name", "slug", "language"},
			"properties": map[string]any{
				"name":     map[string]any{"type": "string"},
				"slug":     map[string]any{"type": "string"},
				"language": map[string]any{"type": "string", "enum": []string{"bun", "python"}},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			name, err := toString(args, "name", true)
			if err != nil {
				return nil, err
			}
			slug, err := toString(args, "slug", true)
			if err != nil {
				return nil, err
			}
			lang, err := toString(args, "language", true)
			if err != nil {
				return nil, err
			}
			body := map[string]any{"name": name, "slug": slug, "language": lang}
			var out map[string]any
			if err := r.client.Post(ctx, authHeader, "/v1/snippets", body, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "update_draft",
		Description: "Create a new snippet version from source code.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"snippet_id", "code"},
			"properties": map[string]any{
				"snippet_id":      map[string]any{"type": "string"},
				"code":            map[string]any{"type": "string"},
				"input_schema":    map[string]any{"type": "string"},
				"output_schema":   map[string]any{"type": "string"},
				"timeout_ms":      map[string]any{"type": "integer"},
				"max_memory_mb":   map[string]any{"type": "integer"},
				"max_cpu_percent": map[string]any{"type": "integer"},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			snippetID, err := toString(args, "snippet_id", true)
			if err != nil {
				return nil, err
			}
			code, err := toString(args, "code", true)
			if err != nil {
				return nil, err
			}
			inputSchema, _ := toString(args, "input_schema", false)
			outputSchema, _ := toString(args, "output_schema", false)
			timeoutMs, _ := toInt(args, "timeout_ms", false)
			maxMemoryMB, _ := toInt(args, "max_memory_mb", false)
			maxCPUPercent, _ := toInt(args, "max_cpu_percent", false)

			body := map[string]any{
				"code": code,
			}
			if inputSchema != "" {
				body["input_schema"] = inputSchema
			}
			if outputSchema != "" {
				body["output_schema"] = outputSchema
			}
			if timeoutMs > 0 {
				body["timeout_ms"] = timeoutMs
			}
			if maxMemoryMB > 0 {
				body["max_memory_mb"] = maxMemoryMB
			}
			if maxCPUPercent > 0 {
				body["max_cpu_percent"] = maxCPUPercent
			}

			var out map[string]any
			path := "/v1/snippets/" + url.PathEscape(snippetID) + "/versions"
			if err := r.client.Post(ctx, authHeader, path, body, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "publish_snippet",
		Description: "Publish a snippet version to an environment.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"snippet_id", "version_number", "env"},
			"properties": map[string]any{
				"snippet_id":     map[string]any{"type": "string"},
				"version_number": map[string]any{"type": "integer"},
				"env":            map[string]any{"type": "string", "enum": []string{"dev", "staging", "prod"}},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			snippetID, err := toString(args, "snippet_id", true)
			if err != nil {
				return nil, err
			}
			versionNum, err := toInt(args, "version_number", true)
			if err != nil {
				return nil, err
			}
			env, err := toString(args, "env", true)
			if err != nil {
				return nil, err
			}
			path := fmt.Sprintf("/v1/snippets/%s/versions/%d/publish?env=%s",
				url.PathEscape(snippetID),
				versionNum,
				url.QueryEscape(env),
			)
			var out map[string]any
			if err := r.client.Post(ctx, authHeader, path, map[string]any{}, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "invoke_snippet",
		Description: "Invoke a snippet synchronously, asynchronously, or as a stream.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"tenant_slug", "snippet_slug"},
			"properties": map[string]any{
				"tenant_slug":  map[string]any{"type": "string"},
				"snippet_slug": map[string]any{"type": "string"},
				"env":          map[string]any{"type": "string"},
				"version":      map[string]any{"type": "string"},
				"invoke_mode":  map[string]any{"type": "string", "enum": []string{"sync", "async", "stream"}},
				"callback_url": map[string]any{"type": "string"},
				"input":        map[string]any{},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			tenantSlug, err := toString(args, "tenant_slug", true)
			if err != nil {
				return nil, err
			}
			snippetSlug, err := toString(args, "snippet_slug", true)
			if err != nil {
				return nil, err
			}
			env, _ := toString(args, "env", false)
			version, _ := toString(args, "version", false)
			mode, _ := toString(args, "invoke_mode", false)
			callbackURL, _ := toString(args, "callback_url", false)

			rawInput, err := toRawInput(args)
			if err != nil {
				return nil, err
			}

			path := fmt.Sprintf("/v1/invoke/%s/%s", url.PathEscape(tenantSlug), url.PathEscape(snippetSlug))
			query := url.Values{}
			if env != "" {
				query.Set("env", env)
			}
			if version != "" {
				query.Set("version", version)
			}
			if encoded := query.Encode(); encoded != "" {
				path += "?" + encoded
			}

			var out map[string]any
			requestBody := any(json.RawMessage(rawInput))
			if callbackURL != "" {
				requestBody = map[string]any{"callback_url": callbackURL}
			}

			if mode == "" || mode == "sync" {
				if err := r.client.Post(ctx, authHeader, path, requestBody, &out); err != nil {
					return nil, err
				}
				return out, nil
			}

			var response map[string]any
			err = r.client.PostWithHeaders(
				ctx,
				authHeader,
				map[string]string{"X-Invoke-Mode": mode},
				path,
				requestBody,
				&response,
			)
			if err != nil {
				return nil, err
			}
			return response, nil
		},
	})

	r.add(Tool{
		Name:        "get_logs",
		Description: "Get invocation logs for a snippet.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"snippet_id"},
			"properties": map[string]any{
				"snippet_id": map[string]any{"type": "string"},
				"limit":      map[string]any{"type": "integer"},
				"status":     map[string]any{"type": "string"},
				"env":        map[string]any{"type": "string"},
				"start_time": map[string]any{"type": "string"},
				"end_time":   map[string]any{"type": "string"},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			snippetID, err := toString(args, "snippet_id", true)
			if err != nil {
				return nil, err
			}
			limit, _ := toInt(args, "limit", false)
			status, _ := toString(args, "status", false)
			env, _ := toString(args, "env", false)
			startTime, _ := toString(args, "start_time", false)
			endTime, _ := toString(args, "end_time", false)

			query := map[string]string{
				"status":     status,
				"env":        env,
				"start_time": startTime,
				"end_time":   endTime,
			}
			if limit > 0 {
				query["limit"] = fmt.Sprintf("%d", limit)
			}
			path := "/v1/logs/snippets/" + url.PathEscape(snippetID) + controlplane.Query(query)
			var out map[string]any
			if err := r.client.Get(ctx, authHeader, path, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "list_secrets",
		Description: "List secret metadata for the authenticated tenant.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
		Handle: func(ctx context.Context, authHeader string, _ map[string]any) (any, error) {
			var out []map[string]any
			if err := r.client.Get(ctx, authHeader, "/v1/secrets", &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "set_secret",
		Description: "Create a secret.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"name", "value"},
			"properties": map[string]any{
				"name":         map[string]any{"type": "string"},
				"value":        map[string]any{"type": "string"},
				"snippet_id":   map[string]any{"type": "string"},
				"environments": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			name, err := toString(args, "name", true)
			if err != nil {
				return nil, err
			}
			value, err := toString(args, "value", true)
			if err != nil {
				return nil, err
			}
			snippetID, _ := toString(args, "snippet_id", false)
			environments, err := toStringSlice(args, "environments")
			if err != nil {
				return nil, err
			}

			body := map[string]any{
				"name":  name,
				"value": value,
			}
			if snippetID != "" {
				body["snippet_id"] = snippetID
			}
			if environments != nil {
				body["environments"] = environments
			}
			var out map[string]any
			if err := r.client.Post(ctx, authHeader, "/v1/secrets", body, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})

	r.add(Tool{
		Name:        "get_metrics",
		Description: "Get aggregate and time-series metrics for a snippet.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"snippet_id"},
			"properties": map[string]any{
				"snippet_id": map[string]any{"type": "string"},
				"window":     map[string]any{"type": "string", "enum": []string{"1h", "24h", "7d"}},
			},
		},
		Handle: func(ctx context.Context, authHeader string, args map[string]any) (any, error) {
			snippetID, err := toString(args, "snippet_id", true)
			if err != nil {
				return nil, err
			}
			window, _ := toString(args, "window", false)
			query := ""
			if strings.TrimSpace(window) != "" {
				query = "?window=" + url.QueryEscape(window)
			}
			path := "/v1/metrics/snippets/" + url.PathEscape(snippetID) + query
			var out map[string]any
			if err := r.client.Get(ctx, authHeader, path, &out); err != nil {
				return nil, err
			}
			return out, nil
		},
	})
}
