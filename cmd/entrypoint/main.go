// Copyright 2026 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	defaultCNIBinDir    = "/host/opt/cni/bin"
	defaultSRIOVBinFile = "/usr/bin/sriov"
	executablePerm      = os.FileMode(0o755)
)

func usage() {
	fmt.Fprintf(os.Stderr,
		"This is an entrypoint script for SR-IOV CNI to overlay its\n"+
			"binary into location in a filesystem. The binary file will\n"+
			"be copied to the corresponding directory.\n\n"+
			"./entrypoint\n"+
			"\t-h --help\n"+
			"\t--cni-bin-dir=%s\n"+
			"\t--sriov-bin-file=%s\n"+
			"\t--no-sleep\n",
		defaultCNIBinDir, defaultSRIOVBinFile)
}

func run() int {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cniBinDir   := fs.String("cni-bin-dir", defaultCNIBinDir, "CNI binary destination directory")
	sriovBinFile := fs.String("sriov-bin-file", defaultSRIOVBinFile, "Source sriov binary path")
	noSleep     := fs.Bool("no-sleep", false, "Exit after copying binary without waiting for signal")
	fs.Usage = usage

	err := fs.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to parse flags: %v\n", err)
		return 1
	}

	cniBinDirClean := filepath.Clean(*cniBinDir)
	if !filepath.IsAbs(cniBinDirClean) {
		fmt.Fprintf(os.Stderr, "cni-bin-dir must be an absolute path, got: %s\n", *cniBinDir)
		return 1
	}

	info, err := os.Stat(cniBinDirClean)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cni-bin-dir %q does not exist: %v\n", cniBinDirClean, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "cni-bin-dir %q is not a directory\n", cniBinDirClean)
		return 1
	}

	if _, err := os.Stat(*sriovBinFile); err != nil {
		fmt.Fprintf(os.Stderr, "sriov-bin-file %q does not exist: %v\n", *sriovBinFile, err)
		return 1
	}

	destPath := filepath.Join(cniBinDirClean, filepath.Base(*sriovBinFile))
	if err := copyFile(*sriovBinFile, destPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to copy %q to %q: %v\n", *sriovBinFile, destPath, err)
		return 1
	}

	if *noSleep {
		fmt.Println("SR-IOV CNI binary installed.")
		return 0
	}

	fmt.Println("SR-IOV CNI binary installed, waiting for termination signal.")
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(ch)
	<-ch
	return 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %q: %w", src, err)
	}
	defer func() {
		if cerr := in.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warn: failed to close source file %q: %v\n", src, cerr)
		}
	}()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, executablePerm)
	if err != nil {
		return fmt.Errorf("create destination %q: %w", dst, err)
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy data: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close destination %q: %w", dst, err)
	}
	return nil
}

func main() {
	os.Exit(run())
}
