package app

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/eugenioenko/ttt/internal/dap"
)

func (a *App) activeDebugPeer() *debugRun {
	if a.debugger.selected != nil {
		return a.debugger.selected
	}
	return a.debugger.run
}

func (a *App) runDebugPeer(root, peer *debugRun, target dap.Target, args map[string]any, open dap.OpenChild, deliver func(func()), ready chan<- error) error {
	childrenCtx, cancelChildren := context.WithCancel(peer.ctx)
	var mu sync.Mutex
	var children sync.WaitGroup
	peer.stopChildren = func() {
		mu.Lock()
		cancelChildren()
		mu.Unlock()
		children.Wait()
	}
	defer peer.stopChildren()
	options := dap.SessionOptions{RunInTerminal: func(ctx context.Context, req dap.TerminalRequest) (int, error) {
		return a.runDebugTerminal(ctx, root, peer, req, deliver)
	}}
	options.StartDebugging = func(requestCtx context.Context, request string, config map[string]any, connect dap.OpenChild) error {
		mu.Lock()
		if err := childrenCtx.Err(); err != nil {
			mu.Unlock()
			return err
		}
		children.Add(1)
		mu.Unlock()
		childCtx, cancel := context.WithCancel(childrenCtx)
		child := &debugRun{ctx: childCtx, cancel: cancel, attached: request == "attach", folder: root.folder, file: root.file, languages: root.languages, points: make(chan map[string][]int, 1)}
		childTarget := dap.Target{Type: target.Type, Request: request, Folder: root.folder, Arguments: config}
		started := make(chan error, 1)
		deliver(func() {
			if childCtx.Err() == nil {
				a.debugger.peers = append(a.debugger.peers, child)
				a.queueDebugPoints(root)
			}
		})
		go func() {
			defer children.Done()
			err := a.runDebugPeer(root, child, childTarget, config, connect, deliver, started)
			canceled := childCtx.Err() != nil
			cancel()
			deliver(func() {
				a.removeDebugChild(child, peer)
				if err != nil && !canceled {
					a.StatusError(err.Error())
				}
			})
		}()
		select {
		case err := <-started:
			return err
		case <-requestCtx.Done():
			cancel()
			return requestCtx.Err()
		}
	}
	err := peer.runSession(peer.ctx, func(ctx context.Context, event func(dap.Event)) (*dap.Session, error) {
		return open(ctx, event, options)
	}, target, args, func(s *dap.Session) {
		deliver(func() {
			peer.session = s
			if parent := a.activeDebugPeer(); peer != root && parent.thread == nil {
				a.debugger.selected = peer
			}
			a.refreshDebugControls()
		})
		if ready != nil {
			ready <- nil
			ready = nil
		}
	}, func(tid int, frames []debugFrame) {
		deliver(func() {
			a.debugger.selected = peer
			peer.frames = frames
			a.showDebugStop(peer, tid, frames)
		})
	}, func() {
		deliver(func() {
			if peer.thread == nil {
				return
			}
			peer.generation++
			peer.busy = false
			if a.activeDebugPeer() == peer {
				a.debugResumed(peer)
			} else {
				peer.thread = nil
			}
		})
	})
	if ready != nil {
		failure := err
		if failure == nil {
			failure = errors.New("child session ended before configuration completed")
		}
		ready <- failure
	}
	return err
}

func (a *App) removeDebugChild(child, parent *debugRun) {
	a.debugger.peers = slices.DeleteFunc(a.debugger.peers, func(r *debugRun) bool { return r == child })
	if a.activeDebugPeer() != child {
		return
	}
	if a.selectPausedPeer() {
		return
	}
	a.debugger.selected = parent
	a.clearDebugInspection()
	a.refreshDebugControls()
}

func (a *App) selectPausedPeer() bool {
	for _, other := range append([]*debugRun{a.debugger.run}, a.debugger.peers...) {
		if other != nil && other.thread != nil && other.ctx.Err() == nil {
			a.debugger.selected = other
			a.showDebugStop(other, *other.thread, other.frames)
			return true
		}
	}
	return false
}
