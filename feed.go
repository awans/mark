package mark

import (
	"crypto/rsa"
	"encoding/json"
	"errors"

	"github.com/square/go-jose"
)

// Op is an arbitrary operation
type Op struct {
	Op       string
	OpNum    int
	FeedHash string
	RawBody  json.RawMessage `json:"Body"`
	Body     interface{}     `json:"-"`
}

// MarshalJSON mostly just copies the Body to the RawBody
func (op *Op) MarshalJSON() ([]byte, error) {
	var rawBody []byte
	var err error
	if op.Body != nil {
		rawBody, err = json.Marshal(op.Body)
		if err != nil {
			return nil, err
		}
	} else {
		rawBody = op.RawBody
	}
	obj := make(map[string]interface{})
	obj["OpNum"] = op.OpNum
	obj["Op"] = op.Op
	obj["FeedHash"] = op.FeedHash
	raw := json.RawMessage(rawBody)
	obj["Body"] = &raw
	return json.Marshal(obj)
}

// Feed is a sequence of operations
type Feed struct {
	Ops []Op `json:"ops"`
}

// DeclareKey returns an Op that sets the key for a feed
func DeclareKey(key *rsa.PublicKey) (*Op, error) {
	jwk, err := AsJWK(key)
	if err != nil {
		return nil, err
	}
	return &Op{Op: "declare-key", Body: jwk}, nil
}

// FromBytes inflates a Feed object from binary
func FromBytes(key *rsa.PublicKey, bytes []byte) (*Feed, error) {
	var feed Feed
	if len(bytes) == 0 {

		// bootstrap a new feed
		var ops []Op
		declareKeyOp, err := DeclareKey(key)
		if err != nil {
			return nil, err
		}
		declareKeyOp.OpNum = 0
		feed = Feed{Ops: ops}
		feed.Ops = append(feed.Ops, *declareKeyOp)
		return &feed, nil
	}
	err := json.Unmarshal(bytes, &feed)
	if err != nil {
		return nil, err
	}

	// TODO - verify feed
	return &feed, nil
}

// Append adds an Op to the end of a feed
func (feed *Feed) Append(op Op) error {
	// op.FeedHash = feed.FeedHash()
	op.OpNum = feed.Ops[len(feed.Ops)-1].OpNum + 1
	feed.Ops = append(feed.Ops, op)
	return nil
}

// CurrentKey returns the currently delcared public key
func (feed *Feed) CurrentKey() (*jose.JsonWebKey, error) {
	for i := len(feed.Ops) - 1; i >= 0; i-- {
		op := feed.Ops[i]
		if op.Op == "declare-key" {
			// this case indicates that i should do something better at unmarshal time
			if op.Body != nil {
				return op.Body.(*jose.JsonWebKey), nil
			}
			var jwk jose.JsonWebKey
			err := json.Unmarshal(op.RawBody, &jwk)
			if err != nil {
				return nil, err
			}
			return &jwk, nil
		}
	}
	return nil, errors.New("Feed had no declared key")
}
