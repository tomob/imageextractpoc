package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

func printHelp() {
	fmt.Printf("Usage:\n%v <image-name> <source-file> [<destination-file>]\n", os.Args[0])
}

func readParams() (string, string, string) {
	image, srcFile := os.Args[1], os.Args[2]
	var dstFile string
	if len(os.Args) > 3 {
		dstFile = os.Args[3]
	} else {
		pwd, err := os.Getwd()
		if err != nil {
			os.Exit(2)
		}
		dstFile = filepath.Join(pwd, filepath.Base(srcFile))
	}

	return image, srcFile, dstFile
}

func closeImage(src types.ImageSource) {
	if err := src.Close(); err != nil {
		fmt.Printf("could not close image: %v\n ", err)
	}
}

func commandTimeoutContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	return ctx, cancel
}

func system() *types.SystemContext {
	return &types.SystemContext{
		// SystemRegistriesConfPath: opts.registriesConfPath,
	}
}

func main() {
	if len(os.Args) < 3 {
		printHelp()
		os.Exit(1)
	}

	img, srcFile, dstFile := readParams()

	fmt.Printf("Image: %v %v %v\n", img, srcFile, dstFile)

	ctx, cancel := commandTimeoutContext()
	defer cancel()

	ref, err := alltransports.ParseImageName(img)
	if err != nil {
		fmt.Printf("Could not parse image: %v", err)
		os.Exit(2)
	}
	sys := system()

	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		fmt.Printf("Could not create image reference: %v", err)
		os.Exit(2)
	}
	defer closeImage(src)

	// fmt.Printf("Ref: %+v", src)

	imgCloser, err := image.FromSource(ctx, sys, src)
	// rawManifest, _, err := src.GetManifest(ctx, nil)
	if err != nil {
		fmt.Printf("Error retrieving image: %v", err)
		os.Exit(2)
	}
	defer imgCloser.Close()

	cache := blobinfocache.DefaultCache(sys)

	// spew.Dump(imgCloser.LayerInfos())

	for _, layer := range imgCloser.LayerInfos() {
		fmt.Printf("Reading layer %v", layer.Digest)

		reader, _, err := src.GetBlob(ctx, layer, cache)
		if err != nil {

		}

		gzipReader, err := gzip.NewReader(reader)
		if err != nil {
			gzipReader.Close()
			reader.Close()
			fmt.Printf("Error creating gzip reader: %v", err)
			os.Exit(2)
		}

		tarReader := tar.NewReader(gzipReader)
		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				break // End of archive
			}
			if err != nil {
				gzipReader.Close()
				reader.Close()
				fmt.Printf("Error reading layer: %v", err)
				os.Exit(2)
			}

			if hdr.Name == srcFile {
				file, err := os.Create(dstFile)
				if err != nil {
					gzipReader.Close()
					reader.Close()
					fmt.Printf("Error creating output file: %v", err)
					os.Exit(2)
				}

				if _, err := io.Copy(file, tarReader); err != nil {
					fmt.Printf("Error copying file: %v", err)
					gzipReader.Close()
					reader.Close()
					os.Exit(2)
				}
				gzipReader.Close()
				reader.Close()
				os.Exit(0)
			}
			// spew.Dump(hdr.Name)
		}

		gzipReader.Close()
		reader.Close()
	}
}
