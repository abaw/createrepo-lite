package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

import "fmt"
import "os"
import "errors"
import "log"
import "path/filepath"
import "strings"
import "golang.org/x/net/context"

// hold necessary information to create metadata
type rpmInfo struct {
	Name string
}

func ParseRPMInfo(ts rpmts, path string) (rpmInfo, error) {
	header, err := ts.openRPM(path)
	if err != nil {
		return rpmInfo{}, err
	}
	defer header.close()

	name, err := header.getString("name")
	if err != nil {
		return rpmInfo{}, err
	}
	return rpmInfo{Name: name}, nil
}

func parseRPMFiles(ctx context.Context, in <-chan string) <-chan *packageInfo {
	out := make(chan *packageInfo)

	go func() {
		ts := newTS()
		defer ts.close()
		defer close(out)
		for path := range in {
			select {
			case <-ctx.Done():
				return
			default:
				info, err := ts.parsePackageInfo(path)
				if err != nil {
					log.Println(err.Error())
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- info:
				}
			}
		}
	}()

	return out
}

func findRPMFiles(ctx context.Context, dir string) <-chan string {
	files := make(chan string)

	go func() {
		defer close(files)
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			canceled := func() error {
				return errors.New(fmt.Sprintf("finding RPM files in %s canceled", dir))
			}

			select {
			case <-ctx.Done():
				return canceled()
			default:
				switch {
				case info.IsDir():
					fallthrough
				case !strings.HasSuffix(path, ".rpm"):
					log.Printf("skipped %s\n", path)
					return nil
				}

				select {
				case files <- path:
				case <-ctx.Done():
					return canceled()
				}
			}

			return nil
		})
	}()
	return files
}

func repeatStr(n uint32, x string) []string {
	v := make([]string, n)
	for i, _ := range v {
		v[i] = x
	}

	return v
}

func genMetadata(ctx context.Context, c <-chan *packageInfo) error {
	db, err := sql.Open("sqlite3", "/tmp/primary.sqlite")
	if err != nil {
		return err
	}
	defer db.Close()

	if err = initPrimaryDB(db); err != nil {
		return err
	}

	placeHolders := strings.Join(repeatStr(25, "?"), ",")

	stmt, err := db.Prepare(fmt.Sprintf("INSERT INTO packages (pkgId, name, arch, version, epoch, release, summary, description, url, time_file, time_build, rpm_license, rpm_vendor, rpm_group, rpm_buildhost, rpm_sourcerpm, rpm_header_start, rpm_header_end, rpm_packager, size_package, size_installed, size_archive, location_href, location_base, checksum_type) values (%s)", placeHolders))
	if err != nil {
		return err
	}

	for p := range c {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_, err = stmt.Exec(
			p.checksum,
			p.rpmName,
			p.rpmArch,
			p.rpmVersion,
			p.rpmEpoch,
			p.rpmRelease,
			p.rpmSummary,
			p.rpmDescription,
			p.rpmUrl,
			p.fileTime,
			p.rpmBuildTime,
			p.rpmLicense,
			p.rpmVendor,
			p.rpmGroup,
			p.rpmBuildHost,
			p.rpmSourceRpm,
			p.headerStart,
			p.headerEnd,
			p.rpmPackager,
			p.fileSize,
			p.rpmInstallSize,
			p.rpmArchiveSize,
			"", // location_href
			"", // location_base
			p.checksumType,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		panic("We accept exactly one argument which is a directory.")
	}

	ctx := context.Background()
	files := findRPMFiles(ctx, os.Args[1])
	out := parseRPMFiles(ctx, files)

	err := genMetadata(ctx, out)
	if err != nil {
		panic(err)
	}

	// for info := range out {
	// 	log.Printf("%s:%+v", info.path, info)
	// }
}
