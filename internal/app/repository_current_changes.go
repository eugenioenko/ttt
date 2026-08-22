package app

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/gdamore/tcell/v3"
)

const currentChangesTimeout = 30 * time.Second

type currentChangesInput struct {
	revision string
	statuses []git.FileStatus
	identity [32]byte
}

func newCurrentChangesInput(revision string, statuses []git.FileStatus) currentChangesInput {
	input := currentChangesInput{revision: revision, statuses: append([]git.FileStatus(nil), statuses...)}
	hash := sha256.New()
	write := func(value []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write(value)
	}
	write([]byte(revision))
	for _, status := range statuses {
		write([]byte(status.Status))
		write([]byte(status.OldPath))
		write([]byte(status.Path))
		if status.Staged {
			write([]byte{1})
		} else {
			write([]byte{0})
		}
	}
	copy(input.identity[:], hash.Sum(nil))
	return input
}

func (s *RepositoryState) SetCurrentChangesHandler(handler func(*CurrentChangesResult)) {
	if s == nil || s.closed {
		return
	}
	s.currentChangesHandler = handler
}

func (s *RepositoryState) SetCurrentChangesRoot(dir, tabID string) uint64 {
	if s == nil || s.closed {
		return 0
	}
	if dir == s.currentRoot && tabID == s.currentTabID {
		return s.currentEpoch
	}
	s.stopCurrentChanges()
	if dir == "" || tabID == "" {
		return 0
	}
	s.currentRoot = dir
	s.currentTabID = tabID
	s.currentEpoch++
	s.currentDirty = true
	_, s.currentInputReady = s.currentInputs[dir]
	s.dirty |= RepositoryCurrentChanges
	s.currentRequest++
	return s.currentEpoch
}

func (s *RepositoryState) EnsureCurrentChanges() {
	if s == nil || s.closed || s.currentRoot == "" || s.currentInFlight || !s.currentDirty {
		return
	}
	if s.currentInputReady {
		s.startCurrentChanges()
		return
	}
	s.RefreshNow(RepositoryWorktree)
}

func (s *RepositoryState) CloseCurrentChangesTab(tabID string) {
	if s == nil || s.closed || tabID == "" || tabID != s.currentTabID {
		return
	}
	s.stopCurrentChanges()
}

func (s *RepositoryState) reportCurrentChangesStatusError(err error) {
	if s == nil || s.closed || err == nil || s.currentRoot == "" {
		return
	}
	if s.currentCancel != nil {
		s.currentCancel()
		s.currentCancel = nil
	}
	s.currentRequest++
	s.currentInFlight = false
	s.currentInFlightRequest = 0
	s.currentInFlightIdentity = [32]byte{}
	s.currentRefreshAgain = false
	s.currentDirty = true
	s.currentInputReady = false
	s.currentAppliedFingerprint = ""
	s.dirty |= RepositoryCurrentChanges
	if s.currentChangesHandler != nil {
		s.currentChangesHandler(&CurrentChangesResult{
			Dir: s.currentRoot, TabID: s.currentTabID, Epoch: s.currentEpoch, Request: s.currentRequest, Err: err,
		})
	}
}

func (s *RepositoryState) stopCurrentChanges() {
	if s == nil {
		return
	}
	if s.currentCancel != nil {
		s.currentCancel()
		s.currentCancel = nil
	}
	s.currentRoot = ""
	s.currentTabID = ""
	s.currentEpoch++
	s.currentRequest++
	s.currentInFlight = false
	s.currentInFlightRequest = 0
	s.currentInFlightIdentity = [32]byte{}
	s.currentRefreshAgain = false
	s.currentDirty = false
	s.currentInputReady = false
	s.currentAppliedFingerprint = ""
	s.dirty &^= RepositoryCurrentChanges
}

func (s *RepositoryState) requestCurrentChanges() {
	if s == nil || s.closed || s.currentRoot == "" || !s.currentInputReady {
		return
	}
	s.currentDirty = true
	s.dirty |= RepositoryCurrentChanges
	if s.currentInFlight {
		s.currentRefreshAgain = true
		input, ok := s.currentInputs[s.currentRoot]
		if ok && input.identity != s.currentInFlightIdentity && s.currentRequest == s.currentInFlightRequest {
			s.currentRequest++
		}
		if s.currentRequest != s.currentInFlightRequest && s.currentCancel != nil {
			s.currentCancel()
		}
		return
	}
	s.currentRequest++
	s.startCurrentChanges()
}

func (s *RepositoryState) startCurrentChanges() {
	if s.closed || s.currentInFlight || s.currentRoot == "" || !s.currentDirty {
		return
	}
	input, ok := s.currentInputs[s.currentRoot]
	if !ok {
		return
	}
	statuses := append([]git.FileStatus(nil), input.statuses...)
	dir, tabID := s.currentRoot, s.currentTabID
	epoch, request := s.currentEpoch, s.currentRequest
	read := s.readCurrentChanges
	ctx, cancel := context.WithTimeout(context.Background(), currentChangesTimeout)
	s.currentCancel = cancel
	s.currentInFlight = true
	s.currentInFlightRequest = request
	s.currentInFlightIdentity = input.identity
	s.currentRefreshAgain = false
	if s.poster == nil {
		result := read(ctx, dir, input.revision, tabID, epoch, request, statuses)
		cancel()
		s.HandleCurrentChanges(result)
		return
	}
	poster := s.poster
	go func() {
		result := read(ctx, dir, input.revision, tabID, epoch, request, statuses)
		cancel()
		_ = poster.PostEvent(tcell.NewEventInterrupt(result))
	}()
}

func (s *RepositoryState) HandleCurrentChanges(result *CurrentChangesResult) bool {
	if s == nil || result == nil || s.closed {
		return false
	}
	if result.Request == s.currentInFlightRequest {
		s.currentInFlight = false
		s.currentInFlightRequest = 0
		s.currentInFlightIdentity = [32]byte{}
		s.currentCancel = nil
	}
	if result.Epoch != s.currentEpoch || result.Dir != s.currentRoot || result.TabID != s.currentTabID || result.Request != s.currentRequest {
		if !s.currentInFlight && s.currentRefreshAgain {
			s.startCurrentChanges()
		}
		return false
	}
	if result.Canceled || errors.Is(result.Err, context.Canceled) {
		if s.currentRefreshAgain {
			s.startCurrentChanges()
		}
		return false
	}
	followUp := s.currentRefreshAgain
	s.currentRefreshAgain = false
	if result.Err != nil {
		s.currentDirty = true
		s.currentInputReady = false
		s.dirty |= RepositoryCurrentChanges
		s.currentAppliedFingerprint = ""
		if s.currentChangesHandler != nil {
			s.currentChangesHandler(result)
		}
		return true
	}
	s.currentDirty = false
	s.dirty &^= RepositoryCurrentChanges
	applied := result.Fingerprint != s.currentAppliedFingerprint
	if applied {
		s.currentAppliedFingerprint = result.Fingerprint
		if s.currentChangesHandler != nil {
			s.currentChangesHandler(result)
		}
	}
	if followUp && s.currentInputReady {
		s.currentDirty = true
		s.dirty |= RepositoryCurrentChanges
		s.currentRequest++
		s.startCurrentChanges()
	}
	return applied
}
