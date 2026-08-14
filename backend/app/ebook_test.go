package app

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yann0917/dedao-gui/backend/config"
	"github.com/yann0917/dedao-gui/backend/services"
	"github.com/yann0917/dedao-gui/backend/utils"
)

func TestLoginedCookiesReturnsEmptyMapWhenNoActiveUser(t *testing.T) {
	original := config.Instance
	config.Instance = &config.ConfigsData{}
	t.Cleanup(func() {
		config.Instance = original
	})

	got := LoginedCookies()
	if got == nil {
		t.Fatal("LoginedCookies returned nil map")
	}
	if len(got) != 0 {
		t.Fatalf("LoginedCookies = %#v, want empty map", got)
	}
}

func TestFetchEbookChaptersPreservesOrder(t *testing.T) {
	orders := []services.EbookOrders{
		{ChapterID: "chapter-1", PathInEpub: "one.xhtml"},
		{ChapterID: "chapter-2", PathInEpub: "two.xhtml"},
	}
	chapterOneRelease := make(chan struct{})
	chapterTwoDone := make(chan struct{})
	type result struct {
		results utils.SvgContents
		err     error
	}
	resultCh := make(chan result, 1)

	go func() {
		results, err := fetchEbookChapters(context.Background(), orders, 2, func(ctx context.Context, order services.EbookOrders, index int) (*utils.SvgContent, error) {
			switch order.ChapterID {
			case "chapter-1":
				select {
				case <-chapterOneRelease:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			case "chapter-2":
				close(chapterTwoDone)
			default:
				return nil, fmt.Errorf("unexpected chapter %q", order.ChapterID)
			}

			return &utils.SvgContent{
				ChapterID:  order.ChapterID,
				PathInEpub: order.PathInEpub,
				OrderIndex: index,
			}, nil
		})
		resultCh <- result{results: results, err: err}
	}()

	select {
	case <-chapterTwoDone:
	case <-time.After(2 * time.Second):
		t.Fatal("chapter two did not complete first")
	}

	close(chapterOneRelease)

	var got result
	select {
	case got = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("fetchEbookChapters did not finish")
	}
	if got.err != nil {
		t.Fatalf("fetchEbookChapters returned error: %v", got.err)
	}

	results := got.results
	if len(results) != len(orders) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(orders))
	}
	if results[0] == nil || results[0].ChapterID != "chapter-1" {
		t.Fatalf("results[0] chapter = %v, want chapter-1", results[0])
	}
	if results[1] == nil || results[1].ChapterID != "chapter-2" {
		t.Fatalf("results[1] chapter = %v, want chapter-2", results[1])
	}
}

func TestFetchEbookChaptersReturnsFirstError(t *testing.T) {
	sentinel := errors.New("chapter-two failed")
	orders := []services.EbookOrders{
		{ChapterID: "chapter-1"},
		{ChapterID: "chapter-2"},
	}
	siblingStarted := make(chan struct{})
	allowFailure := make(chan struct{})
	siblingCanceled := make(chan error, 1)
	errCh := make(chan error, 1)

	go func() {
		_, err := fetchEbookChapters(context.Background(), orders, 2, func(ctx context.Context, order services.EbookOrders, index int) (*utils.SvgContent, error) {
			if order.ChapterID == "chapter-2" {
				select {
				case <-allowFailure:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return nil, sentinel
			}

			close(siblingStarted)
			select {
			case <-ctx.Done():
				siblingCanceled <- ctx.Err()
				return nil, ctx.Err()
			}
		})
		errCh <- err
	}()

	select {
	case <-siblingStarted:
		close(allowFailure)
	case <-time.After(2 * time.Second):
		t.Fatal("sibling callback did not start")
	}
	var err error
	select {
	case err = <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("fetchEbookChapters did not return")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
	select {
	case cancelErr := <-siblingCanceled:
		if !errors.Is(cancelErr, context.Canceled) {
			t.Fatalf("sibling cancellation error = %v, want %v", cancelErr, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked sibling did not observe cancellation")
	}
}

func TestFetchEbookChaptersCancelsPendingWork(t *testing.T) {
	orders := []services.EbookOrders{
		{ChapterID: "chapter-1"},
		{ChapterID: "chapter-2"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		_, err := fetchEbookChapters(ctx, orders, 1, func(ctx context.Context, order services.EbookOrders, index int) (*utils.SvgContent, error) {
			switch order.ChapterID {
			case "chapter-1":
				close(firstStarted)
				<-ctx.Done()
				return nil, ctx.Err()
			case "chapter-2":
				secondStarted <- struct{}{}
				return &utils.SvgContent{ChapterID: order.ChapterID, OrderIndex: index}, nil
			default:
				return nil, errors.New("unexpected chapter")
			}
		})
		errCh <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first chapter did not start")
	}

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fetchEbookChapters did not return after cancellation")
	}

	select {
	case <-secondStarted:
		t.Fatal("pending chapter started after cancellation")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestFetchEbookChaptersRejectsNonPositiveLimit(t *testing.T) {
	var called atomic.Int32
	_, err := fetchEbookChapters(context.Background(), []services.EbookOrders{{ChapterID: "chapter-1"}}, 0, func(ctx context.Context, order services.EbookOrders, index int) (*utils.SvgContent, error) {
		called.Add(1)
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error for non-positive limit")
	}
	if got := called.Load(); got != 0 {
		t.Fatalf("fetch call count = %d, want 0", got)
	}
}

func TestChapterProgressCoordinatorSkipsPrePublishCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var emitted atomic.Int32
	coordinator := newChapterProgressCoordinator(3, func(progress Progress) {
		emitted.Add(1)
	})
	coordinator.beforeEmitHook = cancel

	err := coordinator.emitSuccess(ctx, "chapter-3")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
	if emitted.Load() != 0 {
		t.Fatalf("emit count = %d, want 0", emitted.Load())
	}
	if got := coordinator.completed.Load(); got != 0 {
		t.Fatalf("completed count = %d, want 0", got)
	}
}

func TestChapterProgressCoordinatorRejectsSuccessAfterFailureRecorded(t *testing.T) {
	failure := errors.New("chapter failed")
	var emitted atomic.Int32
	coordinator := newChapterProgressCoordinator(2, func(progress Progress) {
		emitted.Add(1)
	})
	coordinator.recordFailure(failure)

	err := coordinator.emitSuccess(context.Background(), "chapter-2")
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
	if emitted.Load() != 0 {
		t.Fatalf("emit count = %d, want 0", emitted.Load())
	}
	if got := coordinator.completed.Load(); got != 0 {
		t.Fatalf("completed count = %d, want 0", got)
	}
}

func TestChapterProgressCoordinatorEmitCompletesBeforeFailureRecords(t *testing.T) {
	failure := errors.New("chapter failed")
	var emitted atomic.Int32
	emittedDone := make(chan struct{})
	failureReturned := make(chan struct{})
	coordinator := newChapterProgressCoordinator(2, func(progress Progress) {
		emitted.Add(1)
		close(emittedDone)
	})
	coordinator.beforeEmitHook = func() {
		go func() {
			coordinator.recordFailure(failure)
			close(failureReturned)
		}()
		time.Sleep(50 * time.Millisecond)
	}

	err := coordinator.emitSuccess(context.Background(), "chapter-1")
	if err != nil {
		t.Fatalf("emitSuccess error = %v", err)
	}
	select {
	case <-emittedDone:
	case <-time.After(2 * time.Second):
		t.Fatal("emit callback did not run")
	}
	select {
	case <-failureReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("failure recorder did not return after emit completed")
	}
	if emitted.Load() != 1 {
		t.Fatalf("emit count = %d, want 1", emitted.Load())
	}
	if got := coordinator.completed.Load(); got != 1 {
		t.Fatalf("completed count = %d, want 1", got)
	}
	if !errors.Is(coordinator.failure, failure) {
		t.Fatalf("stored failure = %v, want %v", coordinator.failure, failure)
	}
}
