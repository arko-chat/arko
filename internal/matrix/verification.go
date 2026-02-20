package matrix

import (
	"context"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/arko-chat/arko/internal/session"
	"golang.org/x/crypto/hkdf"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type VerificationState struct {
	TransactionID string
	TheirDevice   id.DeviceID
	TheirUser     id.UserID
	Emojis        []VerificationEmoji
	Decimals      [3]uint
	Done          bool
	Cancelled     bool
	CancelReason  string
}

type VerificationEmoji struct {
	Emoji       string
	Description string
}

type sasSession struct {
	mu            sync.Mutex
	txnID         string
	theirUser     id.UserID
	theirDevice   id.DeviceID
	ourPrivateKey *ecdh.PrivateKey
	ourPublicKey  string
	theirKey      string
	commitment    string
	startContent  map[string]any
	sasBytes      []byte
	weStarted     bool
	macReceived   bool
	macKeys       string
	macMap        map[string]string
}

var sasEmojiTable = [64]VerificationEmoji{
	{"🐶", "Dog"}, {"🐱", "Cat"}, {"🦁", "Lion"}, {"🐎", "Horse"},
	{"🦄", "Unicorn"}, {"🐷", "Pig"}, {"🐘", "Elephant"}, {"🐰", "Rabbit"},
	{"🐼", "Panda"}, {"🐓", "Rooster"}, {"🐧", "Penguin"}, {"🐢", "Turtle"},
	{"🐟", "Fish"}, {"🐙", "Octopus"}, {"🦋", "Butterfly"}, {"🌷", "Flower"},
	{"🌳", "Tree"}, {"🌵", "Cactus"}, {"🍄", "Mushroom"}, {"🌏", "Globe"},
	{"🌙", "Moon"}, {"☁️", "Cloud"}, {"🔥", "Fire"}, {"🍌", "Banana"},
	{"🍎", "Apple"}, {"🍓", "Strawberry"}, {"🌽", "Corn"}, {"🍕", "Pizza"},
	{"🎂", "Cake"}, {"❤️", "Heart"}, {"😀", "Smiley"}, {"🤖", "Robot"},
	{"🎩", "Hat"}, {"👓", "Glasses"}, {"🔧", "Spanner"}, {"🎅", "Santa"},
	{"👍", "Thumbs Up"}, {"☂️", "Umbrella"}, {"⌛", "Hourglass"}, {"⏰", "Clock"},
	{"🎁", "Gift"}, {"💡", "Light Bulb"}, {"📕", "Book"}, {"✏️", "Pencil"},
	{"📎", "Paperclip"}, {"✂️", "Scissors"}, {"🔒", "Lock"}, {"🔑", "Key"},
	{"🔨", "Hammer"}, {"☎️", "Telephone"}, {"🏁", "Flag"}, {"🚂", "Train"},
	{"🚲", "Bicycle"}, {"✈️", "Aeroplane"}, {"🚀", "Rocket"}, {"🏆", "Trophy"},
	{"⚽", "Ball"}, {"🎸", "Guitar"}, {"🎺", "Trumpet"}, {"🔔", "Bell"},
	{"⚓", "Anchor"}, {"🎧", "Headphones"}, {"📁", "Folder"}, {"📌", "Pin"},
}

func emojisFromSASBytes(sasBytes []byte) []VerificationEmoji {
	if len(sasBytes) < 6 {
		return nil
	}

	nums := [7]uint8{
		uint8(sasBytes[0] >> 2),
		uint8((sasBytes[0]&0x03)<<4 | sasBytes[1]>>4),
		uint8((sasBytes[1]&0x0f)<<2 | sasBytes[2]>>6),
		uint8(sasBytes[2] & 0x3f),
		uint8(sasBytes[3] >> 2),
		uint8((sasBytes[3]&0x03)<<4 | sasBytes[4]>>4),
		uint8((sasBytes[4]&0x0f)<<2 | sasBytes[5]>>6),
	}

	emojis := make([]VerificationEmoji, 7)
	for i, n := range nums {
		emojis[i] = sasEmojiTable[n]
	}
	return emojis
}

func decimalsFromSASBytes(sasBytes []byte) [3]uint {
	if len(sasBytes) < 5 {
		return [3]uint{}
	}
	return [3]uint{
		(uint(sasBytes[0])<<5 | uint(sasBytes[1])>>3) + 1000,
		(uint(sasBytes[1]&0x07)<<10 | uint(sasBytes[2])<<2 | uint(sasBytes[3])>>6) + 1000,
		(uint(sasBytes[3]&0x3f)<<7 | uint(sasBytes[4])>>1) + 1000,
	}
}

func generateECDHKeyPair() (*ecdh.PrivateKey, string, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	pub := priv.PublicKey().Bytes()
	return priv, base64.RawStdEncoding.EncodeToString(pub), nil
}

func computeSharedSecret(
	ourPrivate *ecdh.PrivateKey,
	theirPublicB64 string,
) ([]byte, error) {
	theirPubBytes, err := base64.RawStdEncoding.DecodeString(theirPublicB64)
	if err != nil {
		theirPubBytes, err = base64.StdEncoding.DecodeString(theirPublicB64)
		if err != nil {
			return nil, fmt.Errorf("decode their public key: %w", err)
		}
	}
	curve := ecdh.X25519()
	theirPub, err := curve.NewPublicKey(theirPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse their public key: %w", err)
	}
	return ourPrivate.ECDH(theirPub)
}

func deriveSASBytes(
	sharedSecret []byte,
	ourUserID, theirUserID string,
	ourDeviceID, theirDeviceID string,
	txnID string,
	ourKey, theirKey string,
	weStarted bool,
	length int,
) ([]byte, error) {
	var info strings.Builder
	info.WriteString("MATRIX_KEY_VERIFICATION_SAS|")
	if weStarted {
		info.WriteString(ourUserID)
		info.WriteString("|")
		info.WriteString(ourDeviceID)
		info.WriteString("|")
		info.WriteString(ourKey)
		info.WriteString("|")
		info.WriteString(theirUserID)
		info.WriteString("|")
		info.WriteString(theirDeviceID)
		info.WriteString("|")
		info.WriteString(theirKey)
	} else {
		info.WriteString(theirUserID)
		info.WriteString("|")
		info.WriteString(theirDeviceID)
		info.WriteString("|")
		info.WriteString(theirKey)
		info.WriteString("|")
		info.WriteString(ourUserID)
		info.WriteString("|")
		info.WriteString(ourDeviceID)
		info.WriteString("|")
		info.WriteString(ourKey)
	}
	info.WriteString("|")
	info.WriteString(txnID)

	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte(info.String()))
	sasBytes := make([]byte, length)
	if _, err := io.ReadFull(hkdfReader, sasBytes); err != nil {
		return nil, fmt.Errorf("HKDF read: %w", err)
	}
	return sasBytes, nil
}

func computeSASMAC(
	sharedSecret []byte,
	input string,
	info string,
) (string, error) {
	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, []byte(info))
	macKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, macKey); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, macKey)
	mac.Write([]byte(input))
	return base64.RawStdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *Manager) getOrCreateSASSession(
	userID string,
) *sasSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sasSessions[userID]; ok {
		return s
	}
	s := &sasSession{}
	m.sasSessions[userID] = s
	return s
}

func (m *Manager) handleVerificationRequest(
	ctx context.Context,
	userID string,
	client *mautrix.Client,
	evt *event.Event,
) {
	m.mu.RLock()
	existing := m.verificationStates[userID]
	m.mu.RUnlock()

	if existing != nil && (existing.Done || existing.Cancelled) {
		return
	}

	if existing != nil && existing.Emojis != nil {
		return
	}

	m.logger.Info("received verification request",
		"user", userID,
		"from", evt.Sender,
	)

	content := evt.Content.AsVerificationRequest()
	if content == nil {
		return
	}

	if evt.Sender.String() != userID {
		m.logger.Info("ignoring verification request from different user",
			"user", userID,
			"from", evt.Sender,
		)
		return
	}

	txnID := content.TransactionID
	if txnID == "" {
		if raw, ok := evt.Content.Raw["transaction_id"].(string); ok {
			txnID = id.VerificationTransactionID(raw)
		}
	}

	sess := m.getOrCreateSASSession(userID)
	sess.mu.Lock()
	sess.txnID = string(txnID)
	sess.theirUser = evt.Sender
	if raw, ok := evt.Content.Raw["from_device"].(string); ok {
		sess.theirDevice = id.DeviceID(raw)
	}
	sess.mu.Unlock()

	m.mu.Lock()
	m.verificationStates[userID] = &VerificationState{
		TransactionID: string(txnID),
		TheirUser:     evt.Sender,
	}
	m.mu.Unlock()

	readyRaw := map[string]any{
		"from_device":    client.DeviceID.String(),
		"methods":        []string{"m.sas.v1"},
		"transaction_id": string(txnID),
	}

	_, err := client.SendToDevice(
		ctx,
		event.ToDeviceVerificationReady,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				evt.Sender: {
					sess.theirDevice: {Raw: readyRaw},
				},
			},
		},
	)
	if err != nil {
		m.logger.Error("failed to send verification ready",
			"user", userID,
			"err", err,
		)
	}
}

func (m *Manager) handleVerificationStart(
	ctx context.Context,
	userID string,
	client *mautrix.Client,
	evt *event.Event,
) {
	m.mu.RLock()
	existing := m.verificationStates[userID]
	m.mu.RUnlock()

	if existing == nil {
		m.logger.Info("ignoring unsolicited verification start",
			"user", userID,
			"from", evt.Sender,
		)
		return
	}

	if existing.Cancelled || existing.Done {
		return
	}

	m.logger.Info("received verification start",
		"user", userID,
		"from", evt.Sender,
	)

	content := evt.Content.AsVerificationStart()
	if content == nil {
		return
	}

	txnID := content.TransactionID
	if txnID == "" {
		if raw, ok := evt.Content.Raw["transaction_id"].(string); ok {
			txnID = id.VerificationTransactionID(raw)
		}
	}

	if existing.TransactionID != "" && existing.TransactionID != string(txnID) {
		m.logger.Info("ignoring verification start with mismatched txn ID",
			"user", userID,
			"expected", existing.TransactionID,
			"got", string(txnID),
		)
		return
	}

	priv, pubB64, err := generateECDHKeyPair()
	if err != nil {
		m.logger.Error("failed to generate ECDH key pair",
			"user", userID,
			"err", err,
		)
		return
	}

	sess := m.getOrCreateSASSession(userID)
	sess.mu.Lock()
	sess.txnID = string(txnID)
	sess.theirUser = evt.Sender
	if raw, ok := evt.Content.Raw["from_device"].(string); ok {
		sess.theirDevice = id.DeviceID(raw)
	}
	sess.weStarted = false
	sess.startContent = evt.Content.Raw
	sess.ourPrivateKey = priv
	sess.ourPublicKey = pubB64

	commitment := sha256.Sum256(
		[]byte(pubB64 + canonicalJSONFromRaw(evt.Content.Raw)),
	)
	commitmentB64 := base64.RawStdEncoding.EncodeToString(commitment[:])
	sess.commitment = commitmentB64
	sess.mu.Unlock()

	m.mu.Lock()
	state := m.verificationStates[userID]
	if state == nil {
		state = &VerificationState{
			TransactionID: string(txnID),
			TheirUser:     evt.Sender,
		}
		m.verificationStates[userID] = state
	}
	state.TransactionID = string(txnID)
	m.mu.Unlock()

	acceptContent := map[string]any{
		"transaction_id":              string(txnID),
		"method":                      "m.sas.v1",
		"key_agreement_protocol":      "curve25519-hkdf-sha256",
		"hash":                        "sha256",
		"message_authentication_code": "hkdf-hmac-sha256.v2",
		"short_authentication_string": []string{"emoji", "decimal"},
		"commitment":                  commitmentB64,
	}

	_, err = client.SendToDevice(
		ctx,
		event.ToDeviceVerificationAccept,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				evt.Sender: {
					sess.theirDevice: {Raw: acceptContent},
				},
			},
		},
	)
	if err != nil {
		m.logger.Error("failed to send verification accept",
			"user", userID,
			"err", err,
		)
	}
}

func (m *Manager) handleVerificationCancel(
	userID string,
	evt *event.Event,
) {
	reason, _ := evt.Content.Raw["reason"].(string)
	m.logger.Info("verification cancelled",
		"user", userID,
		"reason", reason,
	)

	m.mu.Lock()
	state := m.verificationStates[userID]
	if state != nil {
		state.Cancelled = true
		state.CancelReason = reason
	}
	delete(m.sasSessions, userID)
	m.mu.Unlock()
}

func (m *Manager) handleVerificationKey(
	ctx context.Context,
	userID string,
	client *mautrix.Client,
	evt *event.Event,
) {
	theirKeyB64, _ := evt.Content.Raw["key"].(string)
	txnID, _ := evt.Content.Raw["transaction_id"].(string)

	if theirKeyB64 == "" {
		return
	}

	sess := m.getOrCreateSASSession(userID)
	sess.mu.Lock()

	if sess.theirKey != "" {
		sess.mu.Unlock()
		return
	}

	if sess.ourPrivateKey == nil {
		sess.mu.Unlock()
		m.logger.Warn("received verification key before start",
			"user", userID,
		)
		return
	}

	sess.theirKey = theirKeyB64
	if txnID != "" {
		sess.txnID = txnID
	}
	ourPub := sess.ourPublicKey
	ourPriv := sess.ourPrivateKey
	weStarted := sess.weStarted
	theirUserID := sess.theirUser
	theirDeviceID := sess.theirDevice
	currentTxnID := sess.txnID
	sess.mu.Unlock()

	m.logger.Info("received verification key",
		"user", userID,
		"from", evt.Sender,
	)

	_, err := client.SendToDevice(
		ctx,
		event.ToDeviceVerificationKey,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				theirUserID: {
					theirDeviceID: {
						Raw: map[string]any{
							"transaction_id": currentTxnID,
							"key":            ourPub,
						},
					},
				},
			},
		},
	)
	if err != nil {
		m.logger.Error("failed to send verification key",
			"user", userID,
			"err", err,
		)
		return
	}

	sharedSecret, err := computeSharedSecret(ourPriv, theirKeyB64)
	if err != nil {
		m.logger.Error("ECDH failed",
			"user", userID,
			"err", err,
		)
		return
	}

	sasBytes, err := deriveSASBytes(
		sharedSecret,
		userID, theirUserID.String(),
		client.DeviceID.String(), theirDeviceID.String(),
		currentTxnID,
		ourPub, theirKeyB64,
		weStarted,
		6,
	)
	if err != nil {
		m.logger.Error("SAS derivation failed",
			"user", userID,
			"err", err,
		)
		return
	}

	sess.mu.Lock()
	sess.sasBytes = sasBytes
	sess.mu.Unlock()

	emojis := emojisFromSASBytes(sasBytes)

	sasBytes5, _ := deriveSASBytes(
		sharedSecret,
		userID, theirUserID.String(),
		client.DeviceID.String(), theirDeviceID.String(),
		currentTxnID,
		ourPub, theirKeyB64,
		weStarted,
		5,
	)
	decimals := decimalsFromSASBytes(sasBytes5)

	m.mu.Lock()
	state := m.verificationStates[userID]
	if state != nil {
		state.Emojis = emojis
		state.Decimals = decimals
		state.TheirDevice = theirDeviceID
	}
	m.mu.Unlock()
}

func (m *Manager) handleVerificationMAC(
	ctx context.Context,
	userID string,
	client *mautrix.Client,
	evt *event.Event,
) {
	m.mu.RLock()
	existing := m.verificationStates[userID]
	m.mu.RUnlock()
	if existing == nil || existing.Cancelled || existing.Done {
		return
	}

	m.logger.Info("received verification MAC",
		"user", userID,
		"from", evt.Sender,
	)

	sess := m.getOrCreateSASSession(userID)
	sess.mu.Lock()
	macMap, _ := evt.Content.Raw["mac"].(map[string]any)
	keys, _ := evt.Content.Raw["keys"].(string)
	sess.macReceived = true
	sess.macKeys = keys
	sess.macMap = make(map[string]string)
	for k, v := range macMap {
		if s, ok := v.(string); ok {
			sess.macMap[k] = s
		}
	}
	sess.mu.Unlock()
}

func (m *Manager) handleVerificationDone(
	userID string,
	evt *event.Event,
) {
	m.logger.Info("verification done", "user", userID)

	m.mu.Lock()
	state := m.verificationStates[userID]
	if state != nil {
		state.Done = true
	}
	helper := m.cryptoHelpers[userID]
	client := m.clients[userID]
	delete(m.sasSessions, userID)
	m.mu.Unlock()

	if helper != nil && client != nil {
		machine := helper.Machine()
		if machine != nil {
			ctx := context.Background()
			device, err := machine.CryptoStore.GetDevice(
				ctx, id.UserID(userID), client.DeviceID,
			)
			if err == nil && device != nil {
				device.Trust = id.TrustStateCrossSignedVerified
				_ = machine.CryptoStore.PutDevice(
					ctx, id.UserID(userID), device,
				)
			}
		}
	}
}

func (m *Manager) ConfirmVerification(
	ctx context.Context,
	userID string,
) error {
	m.mu.RLock()
	state := m.verificationStates[userID]
	client := m.clients[userID]
	m.mu.RUnlock()

	if state == nil || client == nil {
		return fmt.Errorf("no active verification session")
	}

	sess := m.getOrCreateSASSession(userID)
	sess.mu.Lock()
	theirUser := sess.theirUser
	theirDevice := sess.theirDevice
	theirKey := sess.theirKey
	ourPriv := sess.ourPrivateKey
	txnID := sess.txnID
	sess.mu.Unlock()

	if ourPriv == nil || theirKey == "" {
		return fmt.Errorf("key exchange incomplete")
	}

	sharedSecret, err := computeSharedSecret(ourPriv, theirKey)
	if err != nil {
		return fmt.Errorf("ECDH: %w", err)
	}

	baseInfo := "MATRIX_KEY_VERIFICATION_MAC" +
		userID + client.DeviceID.String() +
		theirUser.String() + string(theirDevice) +
		txnID

	ourDeviceKeyID := fmt.Sprintf("ed25519:%s", client.DeviceID)
	helper := m.GetCryptoHelper(userID)
	var ourEd25519Key string
	if helper != nil && helper.Machine() != nil {
		account := helper.Machine().OwnIdentity()
		ourEd25519Key = string(account.SigningKey)
	}

	macContent := map[string]any{
		"transaction_id": txnID,
	}

	if ourEd25519Key != "" {
		keyMAC, macErr := computeSASMAC(
			sharedSecret,
			ourEd25519Key,
			baseInfo+ourDeviceKeyID,
		)
		if macErr != nil {
			return fmt.Errorf("compute key MAC: %w", macErr)
		}

		keysMAC, macErr := computeSASMAC(
			sharedSecret,
			ourDeviceKeyID,
			baseInfo+"KEY_IDS",
		)
		if macErr != nil {
			return fmt.Errorf("compute keys MAC: %w", macErr)
		}

		macContent["mac"] = map[string]string{
			ourDeviceKeyID: keyMAC,
		}
		macContent["keys"] = keysMAC
	} else {
		macContent["mac"] = map[string]string{}
		macContent["keys"] = ""
	}

	_, err = client.SendToDevice(
		ctx,
		event.ToDeviceVerificationMAC,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				theirUser: {
					theirDevice: {Raw: macContent},
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf("send MAC: %w", err)
	}

	machine := helper.Machine()
	if machine != nil {
		device, devErr := machine.CryptoStore.GetDevice(
			ctx, id.UserID(userID), client.DeviceID,
		)
		if devErr == nil && device != nil {
			device.Trust = id.TrustStateCrossSignedVerified
			_ = machine.CryptoStore.PutDevice(
				ctx, id.UserID(userID), device,
			)
		}
	}

	doneContent := map[string]any{
		"transaction_id": txnID,
	}

	_, err = client.SendToDevice(
		ctx,
		event.ToDeviceVerificationDone,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				theirUser: {
					theirDevice: {Raw: doneContent},
				},
			},
		},
	)
	return err
}

func (m *Manager) CancelVerification(
	ctx context.Context,
	userID string,
) error {
	m.mu.RLock()
	state := m.verificationStates[userID]
	client := m.clients[userID]
	m.mu.RUnlock()

	if state == nil || client == nil {
		return fmt.Errorf("no active verification session")
	}

	cancelContent := map[string]any{
		"transaction_id": state.TransactionID,
		"code":           "m.user",
		"reason":         "User cancelled",
	}

	_, err := client.SendToDevice(
		ctx,
		event.ToDeviceVerificationCancel,
		&mautrix.ReqSendToDevice{
			Messages: map[id.UserID]map[id.DeviceID]*event.Content{
				state.TheirUser: {
					state.TheirDevice: {Raw: cancelContent},
				},
			},
		},
	)

	m.mu.Lock()
	delete(m.verificationStates, userID)
	delete(m.sasSessions, userID)
	m.mu.Unlock()

	return err
}

func canonicalJSONFromRaw(raw map[string]any) string {
	pairs := make([]string, 0, len(raw))
	for k, v := range raw {
		pairs = append(pairs, fmt.Sprintf("%q:%v", k, jsonValue(v)))
	}
	sortStrings(pairs)
	return "{" + strings.Join(pairs, ",") + "}"
}

func jsonValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case map[string]any:
		return canonicalJSONFromRaw(val)
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = jsonValue(item)
		}
		return "[" + strings.Join(parts, ",") + "]"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func (m *Manager) setupCrypto(
	ctx context.Context,
	userID string,
	dbPath string,
) error {
	m.mu.RLock()
	client := m.clients[userID]
	m.mu.RUnlock()

	if client == nil {
		return ErrNoClient
	}

	s, err := session.Get(userID)
	if err != nil {
		return err
	}
	pickleKey := s.PickleKey
	if len(pickleKey) == 0 {
		pickleKey = make([]byte, 32)
		_, err := rand.Read(pickleKey)
		if err != nil {
			return fmt.Errorf("failed to generate pickle key: %w", err)
		}
		session.Update(userID, func(s *session.Session) {
			s.PickleKey = pickleKey
		})
	}

	helper, err := cryptohelper.NewCryptoHelper(
		client, pickleKey, dbPath,
	)
	if err != nil {
		return fmt.Errorf("create crypto helper: %w", err)
	}

	err = helper.Init(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "mismatching device ID") {
			m.logger.Warn("stale crypto DB, removing and retrying",
				"user", userID,
				"path", dbPath,
				"err", err,
			)

			_ = helper.Close()

			m.mu.Lock()
			delete(m.cryptoHelpers, userID)
			m.mu.Unlock()

			client.Store = nil
			client.Crypto = nil

			if removeErr := os.Remove(dbPath); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf(
					"remove stale crypto db: %w", removeErr,
				)
			}

			helper, err = cryptohelper.NewCryptoHelper(
				client, pickleKey, dbPath,
			)
			if err != nil {
				return fmt.Errorf(
					"recreate crypto helper: %w", err,
				)
			}

			if err = helper.Init(ctx); err != nil {
				return fmt.Errorf(
					"reinit crypto helper: %w", err,
				)
			}
		} else {
			return fmt.Errorf("init crypto helper: %w", err)
		}
	}

	helper.DecryptErrorCallback = func(evt *event.Event, decErr error) {
		m.logger.Warn("live decrypt failed, requesting key",
			"user", userID,
			"room", evt.RoomID,
			"event", evt.ID,
			"err", decErr,
		)
		_ = evt.Content.ParseRaw(evt.Type)
		encContent, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
		if ok {
			helper.RequestSession(
				context.Background(),
				evt.RoomID,
				encContent.SenderKey,
				encContent.SessionID,
				evt.Sender,
				encContent.DeviceID,
			)
		}
	}

	machine := helper.Machine()
	if machine != nil {
		err = machine.ShareKeys(ctx, -1)
		if err != nil {
			m.logger.Warn("failed to share initial keys",
				"user", userID,
				"err", err,
			)
		}
	}

	m.mu.Lock()
	m.cryptoHelpers[userID] = helper
	m.mu.Unlock()

	return nil
}

func (m *Manager) GetCryptoHelper(
	userID string,
) *cryptohelper.CryptoHelper {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cryptoHelpers[userID]
}

func (m *Manager) HasCrossSigningKeys(userID string) bool {
	m.mu.RLock()
	helper, ok := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	if !ok || helper == nil {
		return false
	}

	machine := helper.Machine()
	if machine == nil {
		return false
	}

	pubkeys := machine.GetOwnCrossSigningPublicKeys(
		context.Background(),
	)
	return pubkeys != nil
}

func (m *Manager) ClearVerificationState(userID string) {
	m.mu.Lock()
	delete(m.verificationStates, userID)
	delete(m.sasSessions, userID)
	m.mu.Unlock()
}
