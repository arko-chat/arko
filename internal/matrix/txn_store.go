package matrix

import (
	"context"

	"github.com/puzpuzpuz/xsync/v4"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/id"
)

type InMemoryVerificationStore struct {
	txns *xsync.Map[id.VerificationTransactionID, verificationhelper.VerificationTransaction]
}

func NewInMemoryVerificationStore() *InMemoryVerificationStore {
	return &InMemoryVerificationStore{
		txns: xsync.NewMap[id.VerificationTransactionID, verificationhelper.VerificationTransaction](),
	}
}

func (i *InMemoryVerificationStore) DeleteVerification(ctx context.Context, txnID id.VerificationTransactionID) error {
	if _, ok := i.txns.Load(txnID); !ok {
		return verificationhelper.ErrUnknownVerificationTransaction
	}
	return nil
}

func (i *InMemoryVerificationStore) GetVerificationTransaction(ctx context.Context, txnID id.VerificationTransactionID) (verificationhelper.VerificationTransaction, error) {
	if v, ok := i.txns.Load(txnID); !ok {
		return verificationhelper.VerificationTransaction{}, verificationhelper.ErrUnknownVerificationTransaction
	} else {
		return v, nil
	}
}

func (i *InMemoryVerificationStore) SaveVerificationTransaction(ctx context.Context, txn verificationhelper.VerificationTransaction) error {
	i.txns.Store(txn.TransactionID, txn)
	return nil
}

func (i *InMemoryVerificationStore) FindVerificationTransactionForUserDevice(ctx context.Context, userID id.UserID, deviceID id.DeviceID) (verificationhelper.VerificationTransaction, error) {
	ok := false
	found := verificationhelper.VerificationTransaction{}
	i.txns.Range(func(_ id.VerificationTransactionID, value verificationhelper.VerificationTransaction) bool {
		if value.TheirUserID == userID && value.TheirDeviceID == deviceID {
			found = value
			ok = true
			return false
		}

		return true
	})

	if !ok {
		return verificationhelper.VerificationTransaction{}, verificationhelper.ErrUnknownVerificationTransaction
	}

	return found, nil
}

func (i *InMemoryVerificationStore) GetAllVerificationTransactions(ctx context.Context) (txns []verificationhelper.VerificationTransaction, err error) {
	i.txns.Range(func(_ id.VerificationTransactionID, value verificationhelper.VerificationTransaction) bool {
		txns = append(txns, value)
		return true
	})
	return
}
