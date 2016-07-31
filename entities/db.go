package entities

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/awans/mark/feed"
	"github.com/nu7hatch/gouuid"
	"github.com/square/go-jose"
)

// DB is the access point to the entity DB
type DB struct {
	store Store
	fp    []byte
	c     *feed.Coder
	key   *rsa.PrivateKey // maybe hide this
}

// NewQuery is not implemented yet
func (db *DB) NewQuery(kind string) *Query {
	return &Query{db: db, kind: kind, limit: -1, offset: -1}
}

// ConvertDatoms implements Converter
func ConvertDatoms(bytes []byte) (interface{}, error) {
	var ds []Datom
	err := json.Unmarshal(bytes, &ds)
	return ds, err
}

// ConvertJWK implements Converter
func ConvertJWK(bytes []byte) (interface{}, error) {
	var jwk jose.JsonWebKey
	err := json.Unmarshal(bytes, &jwk)
	return &jwk, err
}

// NewDB is a constructor for a db
func NewDB(store Store, fp []byte, key *rsa.PrivateKey) *DB {
	c := feed.NewCoder()
	c.RegisterOp("eav", ConvertDatoms)
	c.RegisterOp("declare-key", ConvertJWK)

	return &DB{store: store, fp: fp, c: c, key: key}
}

// Close closes the db
func (db *DB) Close() {
	db.store.Close()
}

// RebuildIndexes deletes all keys in the eav indexes and then loads each feed
// TODO rewrite this using GetFeeds
func (db *DB) RebuildIndexes() error {
	for _, index := range []string{"eav", "aev", "ave", "vae"} {
		p, err := db.store.Prefix(NewKey(index).ToBytes())
		if err != nil {
			return err
		}
		for k, _, err := p.Next(); err == nil; k, _, err = p.Next() {
			db.store.Delete(k)
		}
	}
	fi, err := db.store.Prefix(NewKey("feed").ToBytes())
	if err != nil {
		return err
	}
	for _, v, err := fi.Next(); err == nil; _, v, err = fi.Next() {
		var sf feed.SignedFeed
		err = json.Unmarshal(v, &sf)
		if err != nil {
			return err
		}
		feed, err := db.c.Decode(sf)
		if err != nil {
			return err
		}
		db.LoadFeed(feed)
	}
	return nil
}

// LoadFeed applies each op to the db in turn and saves it under the user/feed key
func (db *DB) LoadFeed(feed *feed.Feed) error {
	fp, err := feed.Fingerprint()
	if err != nil {
		return err
	}
	for _, op := range feed.Ops {
		db.applyOp(op, fp)
	}
	return nil
}

func (db *DB) applyOp(op feed.Op, fp []byte) {
	if op.Op != "eav" {
		return
	}
	datoms := op.Body.([]Datom)
	entityIDs := make(map[string]bool)
	for _, datom := range datoms {
		db.applyDatom(datom)
		entityIDs[datom.EntityID] = true
	}
	for entityID := range entityIDs {
		db.ensureSysKeys(entityID, fp)
	}
}

// GetFeeds returns all feed.SignedFeeds
func (db *DB) GetFeeds() ([]feed.SignedFeed, error) {
	var feeds []feed.SignedFeed

	feedK := NewKey("feed")
	i, err := db.store.Prefix(feedK.ToBytes())
	if err != nil {
		return nil, err
	}
	for _, v, err := i.Next(); err == nil; _, v, err = i.Next() {
		var sf feed.SignedFeed
		err = json.Unmarshal(v, &sf)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, sf)
	}
	return feeds, nil
}

// GetFeed returns a single SignedFeed by id
func (db *DB) GetFeed(id string) (feed.SignedFeed, error) {
	feedK := NewKey("feed", string(db.fp))
	feedBytes, err := db.store.Get(feedK.ToBytes())
	if err != nil {
		return nil, err
	}
	var sf feed.SignedFeed
	err = json.Unmarshal(feedBytes, &sf)
	if err != nil {
		return nil, err
	}
	return sf, nil
}

// UserFeed loads the feed for the user in this session
func (db *DB) UserFeed() (*feed.Feed, error) {
	feedK := NewKey("feed", string(db.fp))
	feedBytes, err := db.store.Get(feedK.ToBytes())
	if err != nil {
		return nil, err
	}
	var sf feed.SignedFeed
	err = json.Unmarshal(feedBytes, &sf)
	if err != nil {
		return nil, err
	}
	return db.c.Decode(sf)
}

// PutUserFeed sets a user's feed in the db
func (db *DB) PutUserFeed(f *feed.Feed) (feed.SignedFeed, error) {
	sf, err := db.c.Encode(f, db.key)
	if err != nil {
		return nil, err
	}
	return sf, db.PutFeed(sf)
}

// PutFeed sets a feed in the store
func (db *DB) PutFeed(sf feed.SignedFeed) error {
	fp, err := sf.Fingerprint()
	if err != nil {
		return err
	}
	feedBytes, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	feedK := Key{path: [][]byte{[]byte("feed"), fp}}
	db.store.Set(feedK.ToBytes(), feedBytes)
	db.RebuildIndexes() // TODO this is too much work
	return nil
}

// GetPubs returns all Pubs this node knows about
func (db *DB) GetPubs() ([]feed.Pub, error) {
	var pubs []feed.Pub

	pubK := NewKey("pub")
	i, err := db.store.Prefix(pubK.ToBytes())
	if err != nil {
		return nil, err
	}
	for _, v, err := i.Next(); err == nil; _, v, err = i.Next() {
		var pub feed.Pub
		err = json.Unmarshal(v, &pub)
		if err != nil {
			return nil, err
		}
		pubs = append(pubs, pub)
	}
	return pubs, nil
}

// PutPub adds a pub to the collection this node knows about
func (db *DB) PutPub(p *feed.Pub) error {
	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	k := NewKey("pub", string(p.URLHash()))
	db.store.Set(k.ToBytes(), bytes)
	return nil
}

// PutSelf sets the Pub that is this node
func (db *DB) PutSelf(p *feed.Pub) error {
	k := NewKey("pub", "self")
	bytes, err := json.Marshal(p)
	if err != nil {
		return err
	}
	db.store.Set(k.ToBytes(), bytes)
	return nil
}

// GetSelf returns the Pub that is this node
func (db *DB) GetSelf() (*feed.Pub, error) {
	pubK := NewKey("pub", "self")
	bytes, err := db.store.Get(pubK.ToBytes())
	if err != nil {
		return nil, err
	}
	var pub feed.Pub
	err = json.Unmarshal(bytes, &pub)
	return &pub, err
}

func (db *DB) applyDatom(d Datom) {
	// eav, aev, ave, vae
	// we probably don't need all of these..
	if d.Added {
		// TODO value shouldn't always have to be a string
		// maybe store it as gob?
		db.store.Set(d.EAVKey(), []byte(fmt.Sprintf("%s", d.Value)))
		db.store.Set(d.AEVKey(), []byte(fmt.Sprintf("%s", d.Value)))
		db.store.Set(d.AVEKey(), []byte(d.EntityID))
		db.store.Set(d.VAEKey(), []byte(d.EntityID))
	} else {
		db.store.Delete(d.EAVKey())
		db.store.Delete(d.AEVKey())
		db.store.Delete(d.AVEKey())
		db.store.Delete(d.VAEKey())
	}
}

func (db *DB) ensureSysKeys(entityID string, fp []byte) {
	fd := Datom{
		EntityID:  entityID,
		Attribute: "db/FeedID",
		Value:     fp,
		Added:     true,
	}
	db.applyDatom(fd)
	idd := Datom{
		EntityID:  entityID,
		Attribute: "db/ID",
		Value:     entityID,
		Added:     true,
	}
	db.applyDatom(idd)
}

func getKindFromSlicePtr(slice interface{}) string {
	return reflect.ValueOf(slice).Elem().Type().Elem().Name()
}

func getKindFromInstance(instance interface{}) string {
	return reflect.ValueOf(instance).Type().Elem().Name()
}

// GetAll returns all entities of a given type
func (db *DB) GetAll(dst interface{}) error {
	kind := getKindFromSlicePtr(dst)
	prefix := NewKey("ave", "db/Kind", kind)
	i, err := db.store.Prefix(prefix.ToBytes())
	if err != nil {
		return err
	}

	var entityIDs []string
	for _, v, err := i.Next(); err == nil; _, v, err = i.Next() {
		entityIDs = append(entityIDs, string(v))
	}
	db.GetMulti(entityIDs, dst)
	return nil
}

// Get returns a single entity by id
func (db *DB) Get(id string, dst interface{}) error {
	prefix := NewKey("eav", id)
	i, err := db.store.Prefix(prefix.ToBytes())
	if err != nil {
		return err
	}

	entityType := reflect.ValueOf(dst).Elem().Type()
	entity := reflect.New(entityType).Interface()

	for k, v, err := i.Next(); err == nil; k, v, err = i.Next() {
		components := bytes.Split(k, Separator)
		// eav/123/user/name = Andrew
		attr := string(components[3])
		field := reflect.ValueOf(entity).Elem().FieldByName(attr)
		sv := string(v)
		if field.IsValid() {
			field.Set(reflect.ValueOf(sv))
		}
	}
	reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(entity).Elem())
	return nil
}

// GetMulti fetches many keys
// dst is a pointer to a slice
func (db *DB) GetMulti(ids []string, dst interface{}) error {
	v := reflect.ValueOf(dst).Elem() // v is a Value(sliceInstance)
	entityType := v.Type().Elem()    // v is a V(sliceInstance)->T(sliceType)->T(inner type)

	for _, id := range ids {
		entity := reflect.New(entityType).Interface()
		db.Get(id, entity)
		v.Set(reflect.Append(v, reflect.ValueOf(entity).Elem()))
	}
	return nil
}

func eavOp(datoms []Datom) feed.Op {
	op := feed.Op{Op: "eav", Body: datoms}
	return op
}

func isSysKey(s string) bool {
	return s == "ID" || s == "FeedID"
}

// Put sets src at id
// TODO load it first and store the delta
func (db *DB) Put(id string, src interface{}) error {
	kind := getKindFromInstance(src)
	c := reflect.ValueOf(src).Elem()
	cType := c.Type()

	feed, err := db.UserFeed()
	if err != nil {
		return err
	}

	var datoms []Datom
	kd := Datom{
		EntityID:  id,
		Attribute: "db/Kind",
		Value:     kind,
		Added:     true,
	}
	datoms = append(datoms, kd)

	for i := 0; i < cType.NumField(); i++ {
		valueField := c.Field(i)
		typeField := cType.Field(i)

		if isSysKey(typeField.Name) {
			continue
		}

		attrName := kind + "/" + typeField.Name

		d := Datom{
			EntityID:  id,
			Attribute: attrName,
			Value:     valueField.Interface(),
			Added:     true,
		}
		datoms = append(datoms, d)
	}

	op := eavOp(datoms)
	feed.Append(op)

	sf, err := db.PutUserFeed(feed)
	if err != nil {
		return err
	}
	fp, err := feed.Fingerprint()
	if err != nil {
		return err
	}
	db.applyOp(op, fp)
	db.announce(sf)
	return nil
}

// Add adds a new entity to the db
func (db *DB) Add(src interface{}) (id string, err error) {
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	id = u.String()
	err = db.Put(id, src)
	return id, err
}

func (db *DB) announce(f feed.SignedFeed) error {
	self, err := db.GetSelf()
	if err != nil {
		return err
	}
	pubs, err := db.GetPubs()
	if err != nil {
		return err
	}

	return feed.Announce(self, pubs, f)
}
