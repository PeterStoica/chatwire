package linker_test

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"

	"github.com/PeterStoica/chatwire/internal/linker"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeworld"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type rig struct {
	world   *fakeworld.World
	linker  *linker.Linker
	saved   atomic.Int32
	saveErr error
}

func newRig(t *testing.T) *rig {
	t.Helper()
	w, err := fakeworld.New(21)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{world: w}
	cfg := linkflow.Config{
		Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: w.Random, Now: time.Now,
		Save: func(linkflow.Linked) error {
			r.saved.Add(1)
			return r.saveErr
		},
	}
	r.linker = linker.New(cfg, nil)
	w.Screen = func() string { return r.linker.Status().QR }
	t.Cleanup(r.linker.Close)
	return r
}

func linkedOrOver(s linker.Status) bool {
	return s.Phase == linker.Linked || s.Phase == linker.Expired || s.Phase == linker.Failed
}

func TestPhoneNumberLinkingShowsACodeThenLinksOnceItIsTyped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.CodePairing(), r.world.Login(fakeworld.Success()))
		status, err := r.linker.Start(t.Context(), "+40 700 000 000")
		if err != nil {
			t.Fatal(err)
		}
		if status.Phase != linker.ShowingCode || len(status.Code) != 9 || status.Code[4] != '-' || status.Phone != "40700000000" {
			t.Fatalf("Start() = %+v", status)
		}
		r.world.Type(status.Code)
		final, err := r.linker.Await(t.Context(), linkedOrOver)
		if err != nil {
			t.Fatal(err)
		}
		if final.Phase != linker.Linked || final.Account.JID != r.world.Phone.JID || r.saved.Load() != 1 {
			t.Fatalf("final = %+v, saved %d times", final, r.saved.Load())
		}
		again, err := r.linker.Start(t.Context(), "40711111111")
		if err != nil || again.Phase != linker.Linked || r.world.Dials() != 2 {
			t.Fatalf("Start() after linking = %+v, %v with %d dials", again, err, r.world.Dials())
		}
	})
}

func TestQRLinking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.QRPairing(10*time.Second), r.world.Login(fakeworld.Success()))
		status, err := r.linker.Start(t.Context(), "")
		if err != nil {
			t.Fatal(err)
		}
		if status.Phase != linker.ShowingQR || !strings.HasPrefix(status.QR, "https://wa.me/settings/linked_devices#2@ref-1,") {
			t.Fatalf("Start() = %+v", status)
		}
		if final, err := r.linker.Await(t.Context(), linkedOrOver); err != nil || final.Phase != linker.Linked {
			t.Fatalf("final = %+v, %v", final, err)
		}
	})
}

func TestAskingAgainForTheSameNumberKeepsTheCodeOnThePhone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.CodePairing(), r.world.Login(fakeworld.Success()))
		first, err := r.linker.Start(t.Context(), "40700000000")
		if err != nil {
			t.Fatal(err)
		}
		second, err := r.linker.Start(t.Context(), "+40 (700) 000-000")
		if err != nil {
			t.Fatal(err)
		}
		if second.Code != first.Code || len(r.world.ShownCodes()) > 0 {
			t.Fatalf("second request produced %q after %q", second.Code, first.Code)
		}
		r.world.Type(second.Code)
		if final, err := r.linker.Await(t.Context(), linkedOrOver); err != nil || final.Phase != linker.Linked {
			t.Fatalf("final = %+v, %v", final, err)
		}
	})
}

func TestAnotherNumberRestartsLinking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.CodePairing(), r.world.CodePairing(), r.world.Login(fakeworld.Success()))
		first, err := r.linker.Start(t.Context(), "40700000000")
		if err != nil {
			t.Fatal(err)
		}
		second, err := r.linker.Start(t.Context(), "40711111111")
		if err != nil {
			t.Fatal(err)
		}
		if second.Phase != linker.ShowingCode || second.Phone != "40711111111" || second.Code == first.Code {
			t.Fatalf("restart = %+v after %+v", second, first)
		}
		r.world.Type(second.Code)
		if final, err := r.linker.Await(t.Context(), linkedOrOver); err != nil || final.Phase != linker.Linked {
			t.Fatalf("final = %+v, %v", final, err)
		}
	})
}

func TestUnscannedQRExpiresAndTheNextStartBeginsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.Unscanned(), r.world.QRPairing(time.Second), r.world.Login(fakeworld.Success()))
		if _, err := r.linker.Start(t.Context(), ""); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		expired, err := r.linker.Await(t.Context(), linkedOrOver)
		if err != nil || expired.Phase != linker.Expired || !errors.Is(expired.Err, linkflow.ErrQRExpired) {
			t.Fatalf("expected expiry, got %+v, %v", expired, err)
		}
		if waited := time.Since(start); waited != 160*time.Second {
			t.Fatalf("expired after %s, want 160s", waited)
		}
		again, err := r.linker.Start(t.Context(), "")
		if err != nil || again.Phase != linker.ShowingQR {
			t.Fatalf("restart = %+v, %v", again, err)
		}
		if final, err := r.linker.Await(t.Context(), linkedOrOver); err != nil || final.Phase != linker.Linked {
			t.Fatalf("final = %+v, %v", final, err)
		}
	})
}

func TestSaveFailureIsReportedNotSwallowed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.saveErr = errors.New("disk full")
		r.world.Script(r.world.QRPairing(time.Second), r.world.Login(fakeworld.Success()))
		if _, err := r.linker.Start(t.Context(), ""); err != nil {
			t.Fatal(err)
		}
		final, err := r.linker.Await(t.Context(), linkedOrOver)
		if err != nil || final.Phase != linker.Failed || final.Err == nil {
			t.Fatalf("final = %+v, %v", final, err)
		}
	})
}

func TestAlreadyLinkedDoesNotDial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(22)
		if err != nil {
			t.Fatal(err)
		}
		account := pairing.Account{JID: w.Phone.JID}
		l := linker.New(linkflow.Config{Dial: w.Dial}, &account)
		defer l.Close()
		status, err := l.Start(t.Context(), "40700000000")
		if err != nil || status.Phase != linker.Linked || status.Account.JID != w.Phone.JID {
			t.Fatalf("Start() = %+v, %v", status, err)
		}
	})
}

func TestBadPhoneNumberIsRefusedBeforeDialling(t *testing.T) {
	r := newRig(t)
	if _, err := r.linker.Start(t.Context(), "0721 234 567"); !errors.Is(err, pairing.ErrPhone) {
		t.Fatalf("Start() error = %v, want %v", err, pairing.ErrPhone)
	}
	if status := r.linker.Status(); status.Phase != linker.Unlinked {
		t.Fatalf("status = %+v", status)
	}
}

func TestCloseStopsLinkingInFlight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.Unscanned())
		if _, err := r.linker.Start(t.Context(), ""); err != nil {
			t.Fatal(err)
		}
		r.linker.Close()
		if status := r.linker.Status(); status.Phase != linker.ShowingQR {
			t.Fatalf("status after close = %+v", status)
		}
	})
}

func TestWhenLinkedFiresOnlyForASuccessfulLink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var mu sync.Mutex
		var linked []linkflow.Linked
		r.linker.WhenLinked(func(l linkflow.Linked) {
			mu.Lock()
			defer mu.Unlock()
			linked = append(linked, l)
		})
		r.world.Script(r.world.Unscanned(), r.world.QRPairing(time.Second), r.world.Login(fakeworld.Failure("401")), r.world.QRPairing(time.Second), r.world.Login(fakeworld.Success()))
		for _, want := range []linker.Phase{linker.Expired, linker.Failed, linker.Linked} {
			if _, err := r.linker.Start(t.Context(), ""); err != nil {
				t.Fatal(err)
			}
			final, err := r.linker.Await(t.Context(), linkedOrOver)
			if err != nil || final.Phase != want {
				t.Fatalf("final = %+v, %v, want %v", final, err, want)
			}
		}
		synctest.Wait()
		mu.Lock()
		defer mu.Unlock()
		if len(linked) != 1 || linked[0].Account.JID != r.world.Phone.JID {
			t.Fatalf("WhenLinked fired %d times", len(linked))
		}
	})
}

func TestALoggedOutLinkerLinksAgain(t *testing.T) {
	t.Parallel()
	l := linker.New(linkflow.Config{}, &pairing.Account{JID: node.JID{User: "40700000000", Server: node.ServerUser}})
	if l.Status().Phase != linker.Linked {
		t.Fatalf("status = %+v", l.Status())
	}
	cause := errors.New("removed on the phone")
	l.LoggedOut(cause)
	if st := l.Status(); st.Phase != linker.LoggedOut || !errors.Is(st.Err, cause) || st.Phase.InFlight() || st.Phase.String() != "logged out" {
		t.Fatalf("status = %+v", st)
	}
}
