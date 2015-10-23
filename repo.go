package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"path/filepath"
)

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// SQL for initializing databases are copied from createrepo/__init__.py
const (
	repoDBVersion    int    = 10
	sqlInitPrimaryDB string = `
        PRAGMA synchronous="OFF";
        pragma locking_mode="EXCLUSIVE";
        CREATE TABLE conflicts (  name TEXT,  flags TEXT,  epoch TEXT,  version TEXT,  release TEXT,  pkgKey INTEGER );
        CREATE TABLE db_info (dbversion INTEGER, checksum TEXT);
        CREATE TABLE files (  name TEXT,  type TEXT,  pkgKey INTEGER);
        CREATE TABLE obsoletes (  name TEXT,  flags TEXT,  epoch TEXT,  version TEXT,  release TEXT,  pkgKey INTEGER );
        CREATE TABLE packages (  pkgKey INTEGER PRIMARY KEY,  pkgId TEXT,  name TEXT,  arch TEXT,  version TEXT,  epoch TEXT,  release TEXT,  summary TEXT,  description TEXT,  url TEXT,  time_file INTEGER,  time_build INTEGER,  rpm_license TEXT,  rpm_vendor TEXT,  rpm_group TEXT,  rpm_buildhost TEXT,  rpm_sourcerpm TEXT,  rpm_header_start INTEGER,  rpm_header_end INTEGER,  rpm_packager TEXT,  size_package INTEGER,  size_installed INTEGER,  size_archive INTEGER,  location_href TEXT,  location_base TEXT,  checksum_type TEXT);
        CREATE TABLE provides (  name TEXT,  flags TEXT,  epoch TEXT,  version TEXT,  release TEXT,  pkgKey INTEGER );
        CREATE TABLE requires (  name TEXT,  flags TEXT,  epoch TEXT,  version TEXT,  release TEXT,  pkgKey INTEGER , pre BOOL DEFAULT FALSE);
        CREATE INDEX filenames ON files (name);
        CREATE INDEX packageId ON packages (pkgId);
        CREATE INDEX packagename ON packages (name);
        CREATE INDEX pkgconflicts on conflicts (pkgKey);
        CREATE INDEX pkgobsoletes on obsoletes (pkgKey);
        CREATE INDEX pkgprovides on provides (pkgKey);
        CREATE INDEX pkgrequires on requires (pkgKey);
        CREATE INDEX providesname ON provides (name);
        CREATE INDEX requiresname ON requires (name);
        CREATE TRIGGER removals AFTER DELETE ON packages
             BEGIN
             DELETE FROM files WHERE pkgKey = old.pkgKey;
             DELETE FROM requires WHERE pkgKey = old.pkgKey;
             DELETE FROM provides WHERE pkgKey = old.pkgKey;
             DELETE FROM conflicts WHERE pkgKey = old.pkgKey;
             DELETE FROM obsoletes WHERE pkgKey = old.pkgKey;
             END;
`
	sqlInitFilelistsDB string = `
            PRAGMA synchronous="OFF";
            pragma locking_mode="EXCLUSIVE";
            CREATE TABLE db_info (dbversion INTEGER, checksum TEXT);
            CREATE TABLE filelist (  pkgKey INTEGER,  dirname TEXT,  filenames TEXT,  filetypes TEXT);
            CREATE TABLE packages (  pkgKey INTEGER PRIMARY KEY,  pkgId TEXT);
            CREATE INDEX dirnames ON filelist (dirname);
            CREATE INDEX keyfile ON filelist (pkgKey);
            CREATE INDEX pkgId ON packages (pkgId);
            CREATE TRIGGER remove_filelist AFTER DELETE ON packages
                   BEGIN
                   DELETE FROM filelist WHERE pkgKey = old.pkgKey;
                   END;
`
	sqlInitOtherDB string = `
            PRAGMA synchronous="OFF";
            pragma locking_mode="EXCLUSIVE";
            CREATE TABLE changelog (  pkgKey INTEGER,  author TEXT,  date INTEGER,  changelog TEXT);
            CREATE TABLE db_info (dbversion INTEGER, checksum TEXT);
            CREATE TABLE packages (  pkgKey INTEGER PRIMARY KEY,  pkgId TEXT);
            CREATE INDEX keychange ON changelog (pkgKey);
            CREATE INDEX pkgId ON packages (pkgId);
            CREATE TRIGGER remove_changelogs AFTER DELETE ON packages
                 BEGIN
                 DELETE FROM changelog WHERE pkgKey = old.pkgKey;
                 END;
`
)

func initDB(db *sql.DB, sqlCreateTables string) error {
	if _, err := db.Exec(sqlCreateTables); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf("INSERT into db_info values (%d, 'direct_create');", repoDBVersion))
	return err
}

func initPrimaryDB(db *sql.DB) error {
	return initDB(db, sqlInitPrimaryDB)
}

func initFilelistsDB(db *sql.DB) error {
	return initDB(db, sqlInitFilelistsDB)
}

func initOtherDB(db *sql.DB) error {
	return initDB(db, sqlInitOtherDB)
}

type repository struct {
	baseDir string
}

// packageInfo hold the necessary information for a RPM package to create metadata database
type packageInfo struct {
	// path is the absolute path to the RPM
	path string
	// checksum is the checksum of the RPM package
	checksum string
	// checksumType is the type of checksum used to checksum the RPM package
	checksumType string
	fileTime     uint32
	fileSize     uint64
	headerStart  uint64
	headerEnd    uint64

	rpmName    string
	rpmArch    string
	rpmVersion string
	rpmEpoch   string
	rpmRelease string
	// rpmSummary is %{summary} with leading and trailing spaces stripped
	rpmSummary string
	// rpmDescription is %{description} with leading and trailing spaces stripped
	rpmDescription string
	// rpmUrl is the URL of the project if any
	rpmUrl       *string
	rpmBuildTime uint32
	rpmLicense   *string
	rpmVendor    *string
	rpmGroup     *string
	rpmBuildHost *string
	rpmSourceRpm *string
	rpmPackager  *string
	// rpmInstallSize is %{size}
	rpmInstallSize uint64
	// rpmArchiveSize is %{archivesize}
	rpmArchiveSize uint64
}

func (ts rpmts) parsePackageInfo(path string) (*packageInfo, error) {
	var info packageInfo
	var err error

	info.path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	info.fileTime = uint32(fileInfo.ModTime().Unix())
	info.fileSize = uint64(fileInfo.Size())

	info.checksumType = "sha256"
	info.checksum, err = calcFileSha256Sum(path)
	if err != nil {
		return nil, err
	}

	hdr, err := ts.openRPM(path)
	if err != nil {
		return nil, err
	}
	defer hdr.close()

	info.headerStart, info.headerEnd, err = hdr.getHeaderRange()
	if err != nil {
		return nil, err
	}

	info.rpmName, err = hdr.getString("name")
	if err != nil {
		return nil, err
	}

	info.rpmArch, err = hdr.getString("arch")
	if err != nil {
		return nil, err
	}

	info.rpmVersion, err = hdr.getString("version")
	if err != nil {
		return nil, err
	}

	info.rpmEpoch, err = hdr.getString("epoch")
	if err != nil {
		// RPM spec states that we could omit Epoch, but createrepo
		// actually use "0" as the default value if Epoch is not present
		// in a RPM.
		info.rpmEpoch = "0"
	}

	info.rpmRelease, err = hdr.getString("release")
	if err != nil {
		return nil, err
	}

	info.rpmSummary, err = hdr.getString("summary")
	if err != nil {
		return nil, err
	}

	info.rpmDescription, err = hdr.getString("description")
	if err != nil {
		return nil, err
	}

	rpmUrl, err := hdr.getString("url")
	if err == nil {
		info.rpmUrl = &rpmUrl
	}

	rpmBuildTime, err := hdr.getNumber("buildtime")
	if err != nil {
		return nil, err
	}
	info.rpmBuildTime = uint32(rpmBuildTime)

	rpmLicense, err := hdr.getString("license")
	if err == nil {
		info.rpmLicense = &rpmLicense
	}

	rpmVendor, err := hdr.getString("vendor")
	if err == nil {
		info.rpmVendor = &rpmVendor
	}

	rpmGroup, err := hdr.getString("group")
	if err == nil {
		info.rpmGroup = &rpmGroup
	}

	rpmBuildHost, err := hdr.getString("buildhost")
	if err == nil {
		info.rpmBuildHost = &rpmBuildHost
	}
	rpmSourceRpm, err := hdr.getString("sourcerpm")
	if err == nil {
		info.rpmSourceRpm = &rpmSourceRpm
	}
	rpmPackager, err := hdr.getString("packager")
	if err == nil {
		info.rpmPackager = &rpmPackager
	}

	info.rpmInstallSize, err = hdr.getNumber("size")
	if err != nil {
		return nil, err
	}

	info.rpmArchiveSize, err = hdr.getNumber("archivesize")
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// calcFileSha256Sum returns a string(hex) representation of the sha256 checksum of a file
func calcFileSha256Sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()

	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}

	checksum := hash.Sum(nil)
	return fmt.Sprintf("%x", checksum), nil
}
