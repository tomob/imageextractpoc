package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

func printHelp() {
	fmt.Printf("Usage:\n%v <image-name> <source-file> [<destination-file>]\n", os.Args[0])
}

func readParams() (string, string, *os.File) {
	image, srcFile := os.Args[1], os.Args[2]
	var dstFile *os.File
	if len(os.Args) > 3 {
		var err error
		dstFile, err = os.Create(os.Args[3])
		if err != nil {
			fmt.Printf("Error creating output file: %v", err)
			os.Exit(2)
		}
	} else {
		dstFile = os.Stdout
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
	ctx, cancel = context.WithTimeout(ctx, 300*time.Second)
	return ctx, cancel
}

func system() *types.SystemContext {
	return &types.SystemContext{}
}

func readImageSource(ctx context.Context, sys *types.SystemContext, img string) types.ImageSource {
	ref, err := alltransports.ParseImageName(img)
	if err != nil {
		fmt.Printf("Could not parse image: %v", err)
		os.Exit(2)
	}

	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		fmt.Printf("Could not create image reference: %v", err)
		os.Exit(2)
	}

	return src
}

func copyFile(tarReader *tar.Reader, dstFile *os.File) {
	if _, err := io.Copy(dstFile, tarReader); err != nil {
		fmt.Printf("Error copying file: %v", err)
		os.Exit(2)
	}
}

func processLayer(ctx context.Context, sys *types.SystemContext, src types.ImageSource, layer types.BlobInfo, srcFile string, dstFile *os.File, cache types.BlobInfoCache) {
	// fmt.Printf("Reading layer %v\n", layer.Digest)

	var reader io.ReadCloser
	reader, _, err := src.GetBlob(ctx, layer, cache)
	if err != nil {
		fmt.Printf("Could not read layer: %v", err)
		os.Exit(2)
	}

	var gzipReader io.ReadCloser
	gzipReader, err = gzip.NewReader(reader)
	if err != nil {
		fmt.Printf("Error creating gzip reader: %v\n", err)

		// The stream is not gziped. Need to recreate the reader for tar library
		reader.Close()
		gzipReader, _, _ = src.GetBlob(ctx, layer, cache)
	} else {
		defer reader.Close()
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			fmt.Printf("Error reading layer: %v", err)
			os.Exit(2)
		}

		// fmt.Printf("hdr.Name %v %v\n", hdr.Name, hdr.Size)
		if hdr.Name == srcFile {
			copyFile(tarReader, dstFile)
			os.Exit(0)
		}
	}
}

func main() {
	if len(os.Args) < 3 {
		printHelp()
		os.Exit(1)
	}

	img, srcFile, dstFile := readParams()
	defer dstFile.Close()

	ctx, cancel := commandTimeoutContext()
	defer cancel()
	sys := system()

	src := readImageSource(ctx, sys, img)
	defer closeImage(src)

	imgCloser, err := image.FromSource(ctx, sys, src)
	if err != nil {
		fmt.Printf("Error retrieving image: %v", err)
		os.Exit(2)
	}
	defer imgCloser.Close()

	cache := blobinfocache.DefaultCache(sys)

	for _, layer := range imgCloser.LayerInfos() {
		processLayer(ctx, sys, src, layer, srcFile, dstFile, cache)
	}

	fmt.Printf("File %v not found", srcFile)
	os.Exit(3)
}
