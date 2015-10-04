package main

import "fmt"
import "os"
import "errors"
import "log"
import "path/filepath"
import "strings"
import "golang.org/x/net/context"

// hold necessary information to create metadata
type RPMInfo struct {
	Name string
}

func ParseRPMInfo(ts rpmts, path string) (RPMInfo, error) {
	header, err := openRPM(ts, path)
	if err != nil {
		return RPMInfo{}, err
	}
	defer header.close()

	name, err := header.getString(rpmtagName)
	if err != nil {
		return RPMInfo{}, err
	}
	return RPMInfo{Name: name}, nil
}

func parseRPMFiles(ctx context.Context, in <-chan string) <-chan RPMInfo {
	out := make(chan RPMInfo)

	go func() {
		ts := newTS()
		defer freeTS(ts)
		defer close(out)
		for path := range in {
			select {
			case <-ctx.Done():
				return
			default:
				info, err := ParseRPMInfo(ts, path)
				if err != nil {
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

			canceled := func () error {
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

func main() {
	if len(os.Args) != 2 {
		panic("We accept exactly one argument which is a directory.")
	}

	ctx := context.Background()
	files := findRPMFiles(ctx, os.Args[1])
	out := parseRPMFiles(ctx, files)

	for info := range out {
		log.Println(info.Name)
	}
}
