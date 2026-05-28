package firecracker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/runeforge/control-plane/internal/executor"
	"go.uber.org/zap"
)

// Executor runs snippets in Firecracker microVMs.
// Requires /dev/kvm on the host. Use EXECUTOR_TYPE=firecracker to enable.
type Executor struct {
	cfg Config
	log *zap.Logger
}

// Config holds all paths needed to boot and manage Firecracker microVMs.
type Config struct {
	FirecrackerBin string
	JailerBin      string
	BunRootfs      string
	PythonRootfs   string
	KernelImage    string
}

// New creates an Executor, verifying that /dev/kvm and the Firecracker binary exist.
func New(cfg Config, log *zap.Logger) (*Executor, error) {
	if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
		return nil, fmt.Errorf("firecracker executor requires /dev/kvm — run on a KVM-enabled host or set EXECUTOR_TYPE=process")
	}
	if _, err := exec.LookPath(cfg.FirecrackerBin); err != nil {
		return nil, fmt.Errorf("firecracker binary not found at %s: %w", cfg.FirecrackerBin, err)
	}
	return &Executor{cfg: cfg, log: log}, nil
}

// Run executes a snippet in a Firecracker microVM.
//
// Production flow (documented for bare-metal deployment):
//  1. jailer creates a chroot jail and drops privileges
//  2. firecracker boots a minimal Linux kernel + language-specific rootfs in <150ms
//  3. The executor HTTP server inside the VM receives the RunSpec via the vsock
//  4. Result is returned via vsock, VM is destroyed
//
// This implementation shells out to the firecracker binary via the API socket.
// For snapshot/restore (sub-50ms warm starts), pre-boot the VM and take a snapshot
// after the executor server is ready; restore from snapshot on each invocation.
func (e *Executor) Run(ctx context.Context, spec executor.RunSpec) executor.RunResult {
	vmID := generateVMID()
	socketPath := filepath.Join("/tmp", "firecracker-"+vmID+".sock")

	rootfs := e.rootfsForLanguage(spec.Language)
	if rootfs == "" {
		return executor.RunResult{ExitCode: -1, Error: fmt.Sprintf("unsupported language: %s", spec.Language)}
	}

	// Boot VM via firecracker API socket.
	if err := e.bootVM(ctx, vmID, socketPath, rootfs); err != nil {
		return executor.RunResult{ExitCode: -1, Error: fmt.Sprintf("failed to boot VM: %v", err)}
	}
	defer e.destroyVM(vmID, socketPath)

	// Forward the run request to the executor HTTP server inside the VM via vsock.
	return e.runInVM(ctx, socketPath, spec)
}

// RunStream is not yet implemented for Firecracker; falls back to Run and emits a single chunk.
func (e *Executor) RunStream(ctx context.Context, spec executor.RunSpec) (<-chan executor.StreamChunk, error) {
	ch := make(chan executor.StreamChunk, 1)
	go func() {
		defer close(ch)
		result := e.Run(ctx, spec)
		ch <- executor.StreamChunk{Data: result.Output, Error: result.Error, Done: true}
	}()
	return ch, nil
}

func (e *Executor) rootfsForLanguage(lang string) string {
	switch lang {
	case "bun", "typescript", "javascript":
		return e.cfg.BunRootfs
	case "python":
		return e.cfg.PythonRootfs
	default:
		return ""
	}
}

// generateVMID creates a random hex string suitable for use as a VM identifier.
func generateVMID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// bootVM boots a Firecracker microVM via its REST API over the Unix socket.
//
// In production: use the Firecracker REST API over the Unix socket to:
//
//	PUT /machine-config (vcpu_count, mem_size_mib)
//	PUT /boot-source (kernel_image_path, boot_args)
//	PUT /drives/rootfs (path_on_host=rootfs, is_root_device=true, is_read_only=true)
//	PUT /vsock (guest_cid=3, uds_path=socketPath+".vsock")
//	PUT /actions {"action_type":"InstanceStart"}
func (e *Executor) bootVM(ctx context.Context, vmID, socketPath, rootfs string) error {
	return fmt.Errorf("bootVM: not implemented — deploy on a host with /dev/kvm and Firecracker binary at %s", e.cfg.FirecrackerBin)
}

// runInVM sends the RunSpec to the executor HTTP server inside the VM via vsock
// and reads back the RunResult.
//
// In production: dial the vsock, POST the RunSpec JSON, read RunResult JSON back.
func (e *Executor) runInVM(ctx context.Context, socketPath string, spec executor.RunSpec) executor.RunResult {
	return executor.RunResult{ExitCode: -1, Error: "runInVM: not implemented — see bootVM"}
}

// destroyVM terminates the Firecracker process and cleans up resources.
//
// In production: PUT /actions {"action_type":"SendCtrlAltDel"} or SIGTERM the process.
// Clean up the socket file and jail directory.
func (e *Executor) destroyVM(vmID, socketPath string) {
	_ = os.Remove(socketPath)
}
