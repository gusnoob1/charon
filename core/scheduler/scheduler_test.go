// Copyright © 2022 Obol Labs Inc.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of  MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along with
// this program.  If not, see <http://www.gnu.org/licenses/>.

package scheduler_test

import (
	"context"
	"flag"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	eth2v1 "github.com/attestantio/go-eth2-client/api/v1"
	eth2p0 "github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/obolnetwork/charon/app/errors"
	"github.com/obolnetwork/charon/app/eth2wrap"
	"github.com/obolnetwork/charon/core"
	"github.com/obolnetwork/charon/core/scheduler"
	"github.com/obolnetwork/charon/testutil"
	"github.com/obolnetwork/charon/testutil/beaconmock"
)

var integration = flag.Bool("integration", false, "enable integration test, requires BEACON_URL vars.")

// TestIntegration runs an integration test for the Scheduler.
// It expects the above flag to enabled and a BEACON_URL env var.
// It then generates a fake manifest with actual mainnet validators
// and logs 10 duties triggered from them.
func TestIntegration(t *testing.T) {
	if !*integration {
		return
	}

	beaconURL, ok := os.LookupEnv("BEACON_URL")
	if !ok {
		t.Fatal("BEACON_URL env var not set")
	}

	ctx := context.Background()

	eth2Cl, err := eth2wrap.NewMultiHTTP(ctx, time.Second*2, beaconURL)
	require.NoError(t, err)

	// Use random actual mainnet validators
	pubkeys := []core.PubKey{
		"0x914cff835a769156ba43ad50b931083c2dadd94e8359ce394bc7a3e06424d0214922ddf15f81640530b9c25c0bc0d490",
		"0x8dae41352b69f2b3a1c0b05330c1bf65f03730c520273028864b11fcb94d8ce8f26d64f979a0ee3025467f45fd2241ea",
		"0x8ee91545183c8c2db86633626f5074fd8ef93c4c9b7a2879ad1768f600c5b5906c3af20d47de42c3b032956fa8db1a76",
		"0xa8785ecbb5c030e5da6cbbacc3e6cad39dffbc7bcf7f223a12844db8c1182603df99f673157f0d27912a53546e0f64fe",
		"0xb790b322e1cce41c48e3c344cf8d752bdc3cfd51e8eeef44a4bdaac081bc92b53b73e823a9878b5d7a532eb9d9dce1e3",
	}

	s, err := scheduler.New(pubkeys, eth2Cl, false)
	require.NoError(t, err)

	count := 10

	s.SubscribeDuties(func(ctx context.Context, duty core.Duty, set core.DutyDefinitionSet) error {
		for idx, data := range set {
			t.Logf("Duty triggered: vidx=%v slot=%v committee=%v\n", idx, duty.Slot, data.(core.AttesterDefinition).CommitteeIndex)
		}
		count--
		if count == 0 {
			s.Stop()
		}

		return nil
	})

	require.NoError(t, s.Run())
}

//go:generate go test . -run=TestSchedulerWait -count=20

// TestSchedulerWait tests the waitChainStart and waitBeaconSync functions.
func TestSchedulerWait(t *testing.T) {
	tests := []struct {
		Name         string
		GenesisAfter time.Duration
		GenesisErrs  int
		SyncedAfter  time.Duration
		SyncedErrs   int
		WaitSecs     int
	}{
		{
			Name:     "wait for nothing",
			WaitSecs: 0,
		},
		{
			Name:         "wait for genesis",
			GenesisAfter: time.Second * 5,
			WaitSecs:     5,
		},
		{
			Name:        "wait for sync",
			SyncedAfter: time.Second,
			WaitSecs:    60, // We wait in blocks of 1min for sync.
		},
		{
			Name:        "genesis errors",
			GenesisErrs: 3,
			WaitSecs:    15, // We backoff in blocks of 5sec for errors.
		},
		{
			Name:       "synced errors",
			SyncedErrs: 2,
			WaitSecs:   10, // We backoff in blocks of 5sec for errors.
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var t0 time.Time
			clock := newTestClock(t0)
			eth2Cl, err := beaconmock.New()
			require.NoError(t, err)

			eth2Cl.GenesisTimeFunc = func(context.Context) (time.Time, error) {
				var err error
				if test.GenesisErrs > 0 {
					err = errors.New("mock error")
					test.GenesisErrs--
				}

				return t0.Add(test.GenesisAfter), err
			}

			eth2Cl.NodeSyncingFunc = func(context.Context) (*eth2v1.SyncState, error) {
				var err error
				if test.SyncedErrs > 0 {
					err = errors.New("mock error")
					test.SyncedErrs--
				}

				return &eth2v1.SyncState{
					IsSyncing: clock.Now().Before(t0.Add(test.SyncedAfter)),
				}, err
			}

			sched := scheduler.NewForT(t, clock, new(delayer).delay, nil, eth2Cl, false)
			sched.Stop() // Just run wait functions, then quit.
			require.NoError(t, sched.Run())
			require.EqualValues(t, test.WaitSecs, clock.Since(t0).Seconds())
		})
	}
}

//go:generate go test . -run=TestSchedulerDuties -update -clean

// TestSchedulerDuties tests the scheduled duties given a deterministic mock beacon node.
func TestSchedulerDuties(t *testing.T) {
	tests := []struct {
		Name     string
		Factor   int // Determines how duties are spread per epoch
		PropErrs int
		Results  int
	}{
		{
			// All duties grouped in first slot of epoch
			Name:    "grouped",
			Factor:  0,
			Results: 2,
		},
		{
			// All duties spread in first N slots of epoch (N is number of validators)
			Name:    "spread",
			Factor:  1,
			Results: 6,
		},
		{
			// All duties spread in first N slots of epoch (except first proposer errors)
			Name:     "spread_errors",
			Factor:   1,
			PropErrs: 1,
			Results:  5,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Configure beacon mock
			var t0 time.Time

			valSet := beaconmock.ValidatorSetA
			eth2Cl, err := beaconmock.New(
				beaconmock.WithValidatorSet(valSet),
				beaconmock.WithGenesisTime(t0),
				beaconmock.WithDeterministicAttesterDuties(test.Factor),
				beaconmock.WithDeterministicProposerDuties(test.Factor),
			)
			require.NoError(t, err)

			// Wrap ProposerDuties to returns some errors
			origFunc := eth2Cl.ProposerDutiesFunc
			eth2Cl.ProposerDutiesFunc = func(ctx context.Context, epoch eth2p0.Epoch, indices []eth2p0.ValidatorIndex) ([]*eth2v1.ProposerDuty, error) {
				if test.PropErrs > 0 {
					test.PropErrs--
					return nil, errors.New("test error")
				}

				return origFunc(ctx, epoch, indices)
			}

			// get pubkeys for validators to schedule
			pubkeys, err := valSet.CorePubKeys()
			require.NoError(t, err)

			// Construct scheduler
			clock := newTestClock(t0)
			delayer := new(delayer)
			sched := scheduler.NewForT(t, clock, delayer.delay, pubkeys, eth2Cl, false)

			// Only test scheduler output for first N slots, so Stop scheduler (and slotTicker) after that.
			const stopAfter = 3
			slotDuration, err := eth2Cl.SlotDuration(context.Background())
			require.NoError(t, err)
			clock.CallbackAfter(t0.Add(time.Duration(stopAfter)*slotDuration), func() {
				time.Sleep(time.Hour) // Do not let the slot ticker tick anymore.
			})

			// Collect results
			type result struct {
				Time       string
				DutyStr    string    `json:"duty"`
				Duty       core.Duty `json:"-"`
				DutyDefSet map[core.PubKey]string
			}
			var (
				results []result
				mu      sync.Mutex
			)
			sched.SubscribeDuties(func(ctx context.Context, duty core.Duty, set core.DutyDefinitionSet) error {
				// Make result human-readable
				resultSet := make(map[core.PubKey]string)
				for pubkey, def := range set {
					b, err := def.MarshalJSON()
					require.NoError(t, err)
					resultSet[pubkey] = string(b)
				}

				// Add result
				mu.Lock()
				defer mu.Unlock()

				results = append(results, result{
					Duty:       duty,
					DutyStr:    duty.String(),
					DutyDefSet: resultSet,
				})

				if len(results) == test.Results {
					sched.Stop()
				}

				return nil
			})

			// Run scheduler
			require.NoError(t, sched.Run())

			// Add deadlines to results
			deadlines := delayer.get()
			for i := 0; i < len(results); i++ {
				results[i].Time = deadlines[results[i].Duty].UTC().Format("04:05.000")
			}
			// Make result order deterministic
			sort.Slice(results, func(i, j int) bool {
				if results[i].Duty.Slot == results[j].Duty.Slot {
					return results[i].Duty.Type < results[j].Duty.Type
				}

				return results[i].Duty.Slot < results[j].Duty.Slot
			})

			// Assert results
			testutil.RequireGoldenJSON(t, results)
		})
	}
}

func TestScheduler_GetDuty(t *testing.T) {
	// Configure beacon mock
	var t0 time.Time
	valSet := beaconmock.ValidatorSetA
	eth2Cl, err := beaconmock.New(
		beaconmock.WithValidatorSet(valSet),
		beaconmock.WithGenesisTime(t0),
		beaconmock.WithDeterministicAttesterDuties(0),
	)
	require.NoError(t, err)

	// get pubkeys for validators to schedule
	pubkeys, err := valSet.CorePubKeys()
	require.NoError(t, err)

	// Construct scheduler
	clock := newTestClock(t0)
	sched := scheduler.NewForT(t, clock, new(delayer).delay, pubkeys, eth2Cl, false)

	_, err = sched.GetDutyDefinition(context.Background(), core.NewAttesterDuty(0))
	require.ErrorContains(t, err, "epoch not resolved yet")

	_, err = sched.GetDutyDefinition(context.Background(), core.NewBuilderProposerDuty(0))
	require.ErrorContains(t, err, "builder-api not enabled")

	slotDuration, err := eth2Cl.SlotDuration(context.Background())
	require.NoError(t, err)

	clock.CallbackAfter(t0.Add(slotDuration).Add(time.Second), func() {
		res, err := sched.GetDutyDefinition(context.Background(), core.Duty{Slot: 0, Type: core.DutyAttester})

		require.NoError(t, err)

		pubKeys, err := valSet.CorePubKeys()
		require.NoError(t, err)

		for _, pubKey := range pubKeys {
			require.NotNil(t, res[pubKey])
		}

		sched.Stop()
	})

	// Run scheduler
	require.NoError(t, sched.Run())
}

// delayer implements scheduler.delayFunc and records the deadline and returns it immediately.
type delayer struct {
	mu        sync.Mutex
	deadlines map[core.Duty]time.Time
}

func (d *delayer) get() map[core.Duty]time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.deadlines
}

// delay implements scheduler.delayFunc and records the deadline and returns it immediately.
func (d *delayer) delay(duty core.Duty, deadline time.Time) <-chan time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deadlines == nil {
		d.deadlines = make(map[core.Duty]time.Time)
	}
	d.deadlines[duty] = deadline

	resp := make(chan time.Time, 1)
	resp <- deadline

	return resp
}

func newTestClock(now time.Time) *testClock {
	return &testClock{
		now: now,
	}
}

// testClock implements clockwork.Clock and provides a deterministic mock clock
// that is advanced by calls to Sleep or After.
// Note this *does not* support concurrency.
type testClock struct {
	now           time.Time
	callbackAfter time.Time
	callback      func()
}

// CallbackAfter sets a callback function that is called once
// before Sleep returns at or after the time has been reached.
// It is useful to trigger logic "when a certain time has been reached".
func (c *testClock) CallbackAfter(after time.Time, callback func()) {
	c.callbackAfter = after
	c.callback = callback
}

func (c *testClock) After(d time.Duration) <-chan time.Time {
	c.Sleep(d)

	resp := make(chan time.Time, 1)
	resp <- c.Now()

	return resp
}

func (c *testClock) Sleep(d time.Duration) {
	c.now = c.now.Add(d)
	if c.callback == nil || c.now.Before(c.callbackAfter) {
		return
	}
	c.callback()
	c.callback = nil
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) Since(t time.Time) time.Duration {
	return c.now.Sub(t)
}

func (c *testClock) NewTicker(time.Duration) clockwork.Ticker {
	panic("not supported")
}

func (c *testClock) NewTimer(time.Duration) clockwork.Timer {
	panic("not supported")
}
