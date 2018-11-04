package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	AppDataPath string
	AlbumDbPath string
	bucketName  []byte = []byte("albums")
)

// Create local app data directory and initialize database.
func init() {
	AppDataPath = setupAppDataDir()
	AlbumDbPath = filepath.Join(AppDataPath, "albums.db")
}

type Album struct {
	DirName  string
	Contents []string
}

// Update the local album database with albums in target dir, then link
// new albums from source dir.
func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: flaclink <source dir> <target dir>")
		return
	}
	source := filepath.Clean(os.Args[1])
	dest := filepath.Clean(os.Args[2])
	updateAlbumDb(dest)
	linkNewAlbums(source, dest)
}

// Find albums among directories in the top level of musicDir. When an album is found,
// check to see if it's in the database. If not, add it.
func updateAlbumDb(musicDir string) error {
	log.Printf("Updating local DB with flac albums already in target dir %s.", musicDir)
	musicFiles, err := ioutil.ReadDir(musicDir)
	if err != nil {
		log.Fatalf("updateAlbumDb: failed to read directory %s", musicDir)
	}

	db, err := bolt.Open(AlbumDbPath, 0640, &bolt.Options{Timeout: 100 * time.Millisecond})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, file := range musicFiles {
		if !file.IsDir() {
			log.Printf("skipping regular file: %s", file.Name())
			continue
		}
		contentPath := filepath.Join(musicDir, file.Name())
		if isAlbum(contentPath) {
			album := newAlbum(contentPath)
			if !inDb(album, db) {
				log.Printf("Adding existing album to DB: %v.", album.DirName)
				addToDb(album, db)
			}
		}
	}
	return nil
}

// Recursively search for .FLAC files, starting at dirPath. Returns true if any
// .FLAC files are found in dirPath or its descendents.
func isAlbum(dirPath string) bool {
	contents, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Printf("isAlbum: failed to read directory %s", dirPath)
		return false
	}
	for _, file := range contents {
		path := filepath.Join(dirPath, file.Name())
		if file.IsDir() {
			return isAlbum(path)
		}
		if filepath.Ext(path) == (".flac") {
			return true
		}
	}
	return false
}

// Constructor for Album. Called when isAlbum returns true.
func newAlbum(path string) (album Album) {
	album.DirName = filepath.Base(path)
	contents, _ := ioutil.ReadDir(path)
	for _, file := range contents {
		album.Contents = append(album.Contents, file.Name())
	}
	return album
}

// Returns true if album is in db, using gob-encoded album.Conents as key.
func inDb(album Album, db *bolt.DB) bool {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(album.Contents); err != nil {
		log.Fatalf("main:inDb:%v", err)
	}

	keyExists := false
	db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get(buf.Bytes())
		if v != nil {
			keyExists = true
		}
		return nil
	})
	return keyExists
}

// Adds album to db, using gob-encoded album.Contents as key.
func addToDb(album Album, db *bolt.DB) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(album.Contents); err != nil {
		return err
	}
	return db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put(buf.Bytes(), []byte(album.DirName))
	})
}

// Scans sourceDir for albums. When an album is found, checks to see if it already
// exists in the local database, meaning it has already been copied to targetDir.
// If not, the album is hardlinked and added to the local database.
func linkNewAlbums(sourceDir string, targetDir string) {
	log.Printf("Scanning for albums in %s.", sourceDir)
	sourceFiles, err := ioutil.ReadDir(sourceDir)
	db, err := bolt.Open(AlbumDbPath, 0640, &bolt.Options{Timeout: 100 * time.Millisecond})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var regFiles, newAlbums, oldAlbums int

	for _, file := range sourceFiles {
		if !file.IsDir() {
			regFiles++
			continue
		}
		contentPath := filepath.Join(sourceDir, file.Name())
		if isAlbum(contentPath) {
			album := newAlbum(contentPath)
			if !inDb(album, db) {
				log.Printf("Linking album: %s.", file.Name())
				linkAlbum(contentPath, targetDir)
				addToDb(album, db)
				newAlbums++
			} else {
				oldAlbums++
			}
		}
	}
	log.Printf("Skipped %d regular files.", regFiles)
	log.Printf("Linked %d new albums, found %d already in DB or duplicate.", newAlbums, oldAlbums)
}

// Recursively link directory at sourcePath to targetPath.
func linkAlbum(sourcePath string, targetPath string) error {
	sourceDirName := filepath.Base(sourcePath)
	targetDirPath := filepath.Join(targetPath, sourceDirName)

	// copy parent dir
	err := os.Mkdir(targetDirPath, 0775)
	if err != nil {
		log.Fatalf("linkAlbum:copy dir:%s", err)
	}

	sourceContents, _ := ioutil.ReadDir(sourcePath)
	for _, file := range sourceContents {
		// recursively copy subdirectories
		if file.IsDir() {
			subSource := filepath.Join(sourcePath, file.Name())
			linkAlbum(subSource, targetDirPath)
		} else {
			// link files
			sourceFilePath := filepath.Join(sourcePath, file.Name())
			targetFilePath := filepath.Join(targetDirPath, file.Name())
			err := os.Link(sourceFilePath, targetFilePath)
			if err != nil {
				log.Fatalf("linkAlbum:link file:%s", err)
			}
		}
	}
	return nil
}

// Create local app data directory and initialize database.
func setupAppDataDir() string {
	appDataPath := createAppDataDir()
	createAlbumDb(appDataPath)
	return appDataPath
}

// Create local app data directory at ~/.flaclink.
func createAppDataDir() (appDataPath string) {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	appDataPath = filepath.Join(usr.HomeDir, ".flaclink")
	if _, err := os.Stat(appDataPath); os.IsNotExist(err) {
		err = os.Mkdir(appDataPath, 0755)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Created data directory at %s.", appDataPath)
	}
	return appDataPath
}

// Create local album database at appDataPath, if it doesn't already exist.
func createAlbumDb(appDataPath string) {
	albumDbPath := filepath.Join(appDataPath, "albums.db")
	dbOptions := &bolt.Options{Timeout: 100 * time.Millisecond}
	if _, err := os.Stat(albumDbPath); os.IsNotExist(err) {
		// Create db
		db, err := bolt.Open(albumDbPath, 0640, dbOptions)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

		// Create bucket for albums
		err = db.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists(bucketName)
			return err
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Created album database at %s.", albumDbPath)
	} else {
		log.Printf("Found album database at %s.", albumDbPath)
	}
}

// Not called in main program. Useful for debugging.
func printAlbumDb() {
	db, err := bolt.Open(AlbumDbPath, 0640, &bolt.Options{Timeout: 100 * time.Millisecond})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var albumContents []string

	log.Print("Albums in DB: ")
	db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketName).Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			dec := gob.NewDecoder(bytes.NewReader(k))
			err = dec.Decode(&albumContents)
			if err != nil {
				log.Fatalf("printAlbumDb:dec.Decode:%v", err)
			}
			log.Printf("Album dir: %s, Contents: %s", v, albumContents)
		}
		return nil
	})
}
