/*
 * Copyright 2022 Aspect Build Systems, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package buildinfo_test

import (
	"testing"

	"github.com/aspect-build/aspect-gazelle/common/buildinfo"
)

const (
	buildTime = "build time"
	hostName  = "host"
	gitCommit = "git commit"
	gitStatus = "git status"
	release   = "1.2.3"

	version = "1.2.3"
)

func TestNew(t *testing.T) {
	actual := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, release)
	expected := &buildinfo.BuildInfo{
		BuildTime: buildTime,
		HostName:  hostName,
		GitCommit: gitCommit,
		GitStatus: gitStatus,
		Release:   release,
	}
	if *actual != *expected {
		t.Errorf("expected %+v, got %+v", expected, actual)
	}
}

func TestCurrent(t *testing.T) {
	actual := buildinfo.Current()
	expected := &buildinfo.BuildInfo{
		BuildTime: buildinfo.BuildTime,
		HostName:  buildinfo.HostName,
		GitCommit: buildinfo.GitCommit,
		GitStatus: buildinfo.GitStatus,
		Release:   buildinfo.Release,
	}
	if *actual != *expected {
		t.Errorf("expected %+v, got %+v", expected, actual)
	}
}

func TestBuildinfoHasRelease(t *testing.T) {
	t.Run("has a release value", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, release)
		if !bi.HasRelease() {
			t.Error("expected HasRelease() to be true")
		}
	})
	t.Run("does not have a release value", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, "")
		if bi.HasRelease() {
			t.Error("expected HasRelease() to be false")
		}
	})
	t.Run("has pre-stamp release value", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, buildinfo.PreStampRelease)
		if bi.HasRelease() {
			t.Error("expected HasRelease() to be false")
		}
	})
}

func TestBuildinfoIsClean(t *testing.T) {
	t.Run("has a clean git status", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, buildinfo.CleanGitStatus, release)
		if !bi.IsClean() {
			t.Error("expected IsClean() to be true")
		}
	})
	t.Run("does not have a clean git status", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, release)
		if bi.IsClean() {
			t.Error("expected IsClean() to be false")
		}
	})
}

func TestVersion(t *testing.T) {
	t.Run("with release, is clean", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, buildinfo.CleanGitStatus, release)
		if actual := bi.Version(); actual != bi.Release {
			t.Errorf("expected %q, got %q", bi.Release, actual)
		}
	})
	t.Run("with release, is not clean", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, gitStatus, release)
		expected := bi.Release + buildinfo.NotCleanVersionSuffix
		if actual := bi.Version(); actual != expected {
			t.Errorf("expected %q, got %q", expected, actual)
		}
	})
	t.Run("without release", func(t *testing.T) {
		bi := buildinfo.New(buildTime, hostName, gitCommit, buildinfo.CleanGitStatus, "")
		if actual := bi.Version(); actual != buildinfo.NoReleaseVersion {
			t.Errorf("expected %q, got %q", buildinfo.NoReleaseVersion, actual)
		}
	})
}
