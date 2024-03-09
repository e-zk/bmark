package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"log" // todo: remove

	bolt "go.etcd.io/bbolt"
)

// requires global 'dbPath'

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func dbGetAll() ([]Bookmark, error) {
	var bookmarks []Bookmark

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return bookmarks, err
	}
	defer db.Close()

	err = db.Batch(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("bookmarks"))
		if err != nil {
			return err
		}

		c := b.Cursor()

		//for k, v := c.First(); k != nil; k, v = c.Next() {
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			//log.Printf("db :: k=%v, v=%v", k, v)

			var bmark Bookmark

			buf := bytes.Buffer{}
			buf.Write(v)
			g := gob.NewDecoder(&buf)
			err = g.Decode(&bmark)
			if err != nil {
				return err
			}

			bookmarks = append(bookmarks, bmark)
		}

		return nil
	})

	return bookmarks, err
}

func dbSave(bookmark Bookmark) error {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		//b := tx.Bucket([]byte("bookmarks"))
		b, err := tx.CreateBucketIfNotExists([]byte("bookmarks"))
		if err != nil {
			return err
		}

		// increment
		id, _ := b.NextSequence()
		bookmark.Id = int(id)

		// gob encode
		buf := bytes.Buffer{}
		g := gob.NewEncoder(&buf)
		err = g.Encode(bookmark)
		if err != nil {
			return err
		}

		log.Printf("db :: saving %v = %+v", bookmark.Id, buf.Bytes())
		return b.Put(itob(bookmark.Id), buf.Bytes())
	})
}
