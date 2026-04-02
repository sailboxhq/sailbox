package k3s

import (
	"context"
	"fmt"
	"io"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
)

func (o *Orchestrator) ExecTerminal(ctx context.Context, app *model.Application, opts orchestrator.ExecOpts) (orchestrator.TerminalSession, error) {
	ns := appNamespace(app)
	name := appK8sName(app)

	// Find the first running pod
	pods, err := o.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", name),
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no running pods found for %s", name)
	}

	podName := pods.Items[0].Name
	if opts.Container == "" && len(pods.Items[0].Spec.Containers) > 0 {
		opts.Container = pods.Items[0].Spec.Containers[0].Name
	}

	cmd := opts.Command
	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	req := o.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.Container,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(o.config, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	session := &terminalSession{
		stdinR:  nil,
		stdinW:  nil,
		stdoutR: nil,
		stdoutW: nil,
	}

	// Create pipes for stdin/stdout
	session.stdinR, session.stdinW = io.Pipe()
	session.stdoutR, session.stdoutW = io.Pipe()

	// Run the exec in a goroutine
	go func() {
		defer func() { _ = session.stdoutW.Close() }()
		err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdin:             session.stdinR,
			Stdout:            session.stdoutW,
			Stderr:            session.stdoutW, // merge stderr into stdout
			Tty:               opts.TTY,
			TerminalSizeQueue: session,
		})
		if err != nil {
			_, _ = fmt.Fprintf(session.stdoutW, "\r\nSession ended: %v\r\n", err)
		}
	}()

	return session, nil
}

// terminalSession implements orchestrator.TerminalSession and remotecommand.TerminalSizeQueue
type terminalSession struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter

	mu       sync.Mutex
	sizeChan chan remotecommand.TerminalSize
	closed   bool
}

func (s *terminalSession) Read(p []byte) (int, error) {
	return s.stdoutR.Read(p)
}

func (s *terminalSession) Write(p []byte) (int, error) {
	return s.stdinW.Write(p)
}

func (s *terminalSession) Resize(width, height uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sizeChan == nil {
		s.sizeChan = make(chan remotecommand.TerminalSize, 1)
	}
	select {
	case s.sizeChan <- remotecommand.TerminalSize{Width: width, Height: height}:
	default:
	}
	return nil
}

func (s *terminalSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	_ = s.stdinW.Close()
	_ = s.stdinR.Close()
	_ = s.stdoutR.Close()
	if s.sizeChan != nil {
		close(s.sizeChan)
	}
	return nil
}

// Next implements remotecommand.TerminalSizeQueue
func (s *terminalSession) Next() *remotecommand.TerminalSize {
	s.mu.Lock()
	if s.sizeChan == nil {
		s.sizeChan = make(chan remotecommand.TerminalSize, 1)
	}
	ch := s.sizeChan
	s.mu.Unlock()

	size, ok := <-ch
	if !ok {
		return nil
	}
	return &size
}
