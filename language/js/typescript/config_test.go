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

package typescript

import (
	"path"
	"sync"
	"testing"
)

func TestGetTsConfigFromPath(t *testing.T) {
	t.Run("cache hit returns identical pointer", func(t *testing.T) {
		tc := NewTsWorkspace(nil)
		tc.SetTsConfigFile(".", "tests", "", "base.tsconfig.json")

		first := tc.GetTsConfigFile("tests", "")
		if first == nil {
			t.Fatal("first GetTsConfigFile returned nil")
		}
		if second := tc.GetTsConfigFile("tests", ""); first != second {
			t.Errorf("expected cached pointer; got distinct *TsConfig values")
		}
	})

	t.Run("failure leaves InvalidTsconfig sentinel and stays nil on re-call", func(t *testing.T) {
		tc := NewTsWorkspace(nil)
		tc.SetTsConfigFile(".", "tests", "", "does-not-exist.json")

		if c := tc.GetTsConfigFile("tests", ""); c != nil {
			t.Fatalf("expected nil for missing file, got %v", c)
		}

		key := path.Join("tests", "does-not-exist.json")
		if got := tc.cm.configs[key]; got != &InvalidTsconfig {
			t.Errorf("cache entry for %q = %v; want &InvalidTsconfig", key, got)
		}

		if c := tc.GetTsConfigFile("tests", ""); c != nil {
			t.Errorf("expected nil on re-call, got %v", c)
		}
		if got := tc.cm.configs[key]; got != &InvalidTsconfig {
			t.Errorf("cache entry mutated after re-call: %v", got)
		}
	})

	t.Run("concurrent callers see same cached pointer", func(t *testing.T) {
		tc := NewTsWorkspace(nil)
		tc.SetTsConfigFile(".", "tests", "", "base.tsconfig.json")

		const goroutines = 32
		var wg sync.WaitGroup
		results := make([]*TsConfig, goroutines)
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func(i int) {
				defer wg.Done()
				results[i] = tc.GetTsConfigFile("tests", "")
			}(i)
		}
		wg.Wait()

		if results[0] == nil {
			t.Fatal("concurrent GetTsConfigFile returned nil")
		}
		for i := 1; i < goroutines; i++ {
			if results[i] != results[0] {
				t.Errorf("goroutine %d: distinct *TsConfig; want pointer-equal", i)
			}
		}
	})
}
