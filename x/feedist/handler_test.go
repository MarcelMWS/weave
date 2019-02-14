package feedist

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/iov-one/weave"
	"github.com/iov-one/weave/app"
	"github.com/iov-one/weave/errors"
	"github.com/iov-one/weave/store"
	"github.com/iov-one/weave/x"
	"github.com/iov-one/weave/x/cash"
)

func TestHandlers(t *testing.T) {
	_, src := helper.MakeKey()

	addr1 := newAddress()
	addr2 := newAddress()

	rt := app.NewRouter()
	auth := helper.CtxAuth("auth")
	cashBucket := cash.NewBucket()
	ctrl := cash.NewController(cashBucket)
	RegisterRoutes(rt, auth, ctrl)

	qr := weave.NewQueryRouter()
	cash.RegisterQuery(qr)
	RegisterQuery(qr)

	// In below cases, asSeqID(1) is used - this is the address of the
	// first revenue instance created. Sequence is reset for each test
	// case.

	cases := map[string]struct {
		prepareAccounts []account
		actions         []action
		wantAccounts    []account
	}{
		"at least one recipient is required": {
			prepareAccounts: nil,
			wantAccounts:    nil,
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin:      []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{},
					},
					blocksize:    100,
					wantCheckErr: errors.InvalidMsgErr,
				},
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 1, Address: addr1},
						},
					},
					blocksize: 101,
				},
				{
					conditions: []weave.Condition{src},
					msg: &UpdateRevenueMsg{
						RevenueID:  asSeqID(1),
						Recipients: []*Recipient{},
					},
					blocksize:    102,
					wantCheckErr: errors.InvalidMsgErr,
				},
			},
		},
		"revenue not found": {
			prepareAccounts: nil,
			wantAccounts:    nil,
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: []byte("revenue-with-this-id-does-not-exist"),
					},
					blocksize:      100,
					wantCheckErr:   errors.NotFoundErr,
					wantDeliverErr: errors.NotFoundErr,
				},
			},
		},
		"weights are normalized during distribution": {
			prepareAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 7, "BTC")}},
			},
			wantAccounts: []account{
				// All funds must be transferred to the only recipient.
				{address: addr1, coins: x.Coins{coinp(0, 7, "BTC")}},
			},
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							// There is only one recipient with a ridiculously high weight.
							// All funds should be send to this account.
							{Weight: 123456789, Address: addr1},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				{
					conditions:     []weave.Condition{src},
					msg:            &DistributeMsg{RevenueID: asSeqID(1)},
					blocksize:      101,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
		"revenue without an account distributing funds": {
			prepareAccounts: nil,
			wantAccounts:    nil,
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 1, Address: addr1},
							{Weight: 2, Address: addr2},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: asSeqID(1),
					},
					blocksize:      101,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
		"revenue with an account but without enough funds": {
			prepareAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 1, "BTC")}},
			},
			wantAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 1, "BTC")}},
			},
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 1, Address: addr1},
							{Weight: 2, Address: addr2},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: asSeqID(1),
					},
					blocksize:      101,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
		"distribute revenue with a leftover funds": {
			prepareAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 7, "BTC")}},
			},
			wantAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 1, "BTC")}},
				{address: addr1, coins: x.Coins{coinp(0, 2, "BTC")}},
				{address: addr2, coins: x.Coins{coinp(0, 4, "BTC")}},
			},
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 10000, Address: addr1},
							{Weight: 20000, Address: addr2},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: asSeqID(1),
					},
					blocksize:      101,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
		"distribute revenue with an account holding various tickers": {
			prepareAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(0, 3, "BTC"), coinp(7, 0, "ETH")}},
			},
			wantAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(1, 0, "ETH")}},
				{address: addr1, coins: x.Coins{coinp(0, 1, "BTC"), coinp(2, 0, "ETH")}},
				{address: addr2, coins: x.Coins{coinp(0, 2, "BTC"), coinp(4, 0, "ETH")}},
			},
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 1, Address: addr1},
							{Weight: 2, Address: addr2},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: asSeqID(1),
					},
					blocksize:      101,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
		"updating a revenue is distributing the collected funds first": {
			prepareAccounts: []account{
				{address: asSeqID(1), coins: x.Coins{coinp(3, 0, "BTC")}},
			},
			wantAccounts: []account{
				{address: addr1, coins: x.Coins{coinp(2, 0, "BTC")}},
				{address: addr2, coins: x.Coins{coinp(1, 0, "BTC")}},
			},
			actions: []action{
				{
					conditions: []weave.Condition{src},
					msg: &NewRevenueMsg{
						Admin: []byte("f427d624ed29c1fae0e2"),
						Recipients: []*Recipient{
							{Weight: 20, Address: addr1},
							{Weight: 20, Address: addr2},
						},
					},
					blocksize:      100,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				// Issuing an update must distribute first.
				// Distributing 3 BTC equally, means that 1 BTC will be left.
				{
					conditions: []weave.Condition{src},
					msg: &UpdateRevenueMsg{
						RevenueID: asSeqID(1),
						Recipients: []*Recipient{
							{Weight: 1234, Address: addr1},
						},
					},
					blocksize:      102,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
				// After the update, all funds should be moved to addr1
				{
					conditions: []weave.Condition{src},
					msg: &DistributeMsg{
						RevenueID: asSeqID(1),
					},
					blocksize:      103,
					wantCheckErr:   nil,
					wantDeliverErr: nil,
				},
			},
		},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			db := store.MemStore()

			for _, a := range tc.prepareAccounts {
				for _, c := range a.coins {
					if err := ctrl.IssueCoins(db, a.address, *c); err != nil {
						t.Fatalf("cannot issue %q to %x: %s", c, a.address, err)
					}
				}
			}

			for i, a := range tc.actions {
				cache := db.CacheWrap()
				if _, err := rt.Check(a.ctx(), cache, a.tx()); !errors.Is(err, a.wantCheckErr) {
					t.Logf("want: %+v", a.wantCheckErr)
					t.Logf(" got: %+v", err)
					t.Fatalf("action %d check (%T)", i, a.msg)
				}
				cache.Discard()
				if a.wantCheckErr != nil {
					// Failed checks are causing the message to be ignored.
					continue
				}

				if _, err := rt.Deliver(a.ctx(), db, a.tx()); !errors.Is(err, a.wantDeliverErr) {
					t.Logf("want: %+v", a.wantDeliverErr)
					t.Logf(" got: %+v", err)
					t.Fatalf("action %d delivery (%T)", i, a.msg)
				}
			}

			for i, a := range tc.wantAccounts {
				coins, err := ctrl.Balance(db, a.address)
				if err != nil {
					t.Fatalf("cannot get %+v balance: %s", a, err)
				}
				if !coins.Equals(a.coins) {
					t.Logf("want: %+v", a.coins)
					t.Logf("got: %+v", coins)
					t.Errorf("unexpected coins for account #%d (%s)", i, a.address)
				}
			}
		})
	}
}

type account struct {
	address weave.Address
	coins   x.Coins
}

// action represents a single request call that is handled by a handler.
type action struct {
	conditions     []weave.Condition
	msg            weave.Msg
	blocksize      int64
	wantCheckErr   error
	wantDeliverErr error
}

func (a *action) tx() weave.Tx {
	return helper.MockTx(a.msg)
}

func (a *action) ctx() weave.Context {
	ctx := weave.WithHeight(context.Background(), a.blocksize)
	ctx = weave.WithChainID(ctx, "testchain-123")
	return helper.CtxAuth("auth").SetConditions(ctx, a.conditions...)
}

var helper x.TestHelpers

func newAddress() weave.Address {
	_, key := helper.MakeKey()
	return key.Address()
}

// asSeqID returns an ID encoded as if it was generated by the bucket sequence
// call.
func asSeqID(i int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(i))
	return b
}

func coinp(w, f int64, ticker string) *x.Coin {
	c := x.NewCoin(w, f, ticker)
	return &c
}

func TestFindGdc(t *testing.T) {
	cases := map[string]struct {
		want   int32
		values []int32
	}{
		"empty": {
			want:   0,
			values: nil,
		},
		"one element": {
			want:   7,
			values: []int32{7},
		},
		"two elements": {
			want:   3,
			values: []int32{9, 6},
		},
		"three elements": {
			want:   3,
			values: []int32{9, 3, 6},
		},
		"four elements": {
			want:   6,
			values: []int32{12, 6, 18},
		},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			got := findGcd(tc.values...)
			if got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}
