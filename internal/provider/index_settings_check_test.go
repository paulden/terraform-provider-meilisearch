package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These helpers talk to Meilisearch directly rather than going through the
// provider, so that a check failing proves the server state is wrong rather
// than merely that Terraform state and provider code agree with each other.

const (
	testMeilisearchHost = "http://localhost:7700"
	testMeilisearchKey  = "T35T-M45T3R-K3Y"
)

func meilisearchRequest(method, path, body string) ([]byte, error) {
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, testMeilisearchHost+path, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testMeilisearchKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s %s: unexpected status %d: %s", method, path, resp.StatusCode, data)
	}

	return data, nil
}

func indexSettings(uid string) (map[string]interface{}, error) {
	data, err := meilisearchRequest(http.MethodGet, "/indexes/"+uid+"/settings", "")
	if err != nil {
		return nil, err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// canonicalJSON renders a value with string arrays sorted, so that comparisons
// do not depend on the order Meilisearch happens to return.
func canonicalJSON(value interface{}) (string, error) {
	if list, ok := value.([]interface{}); ok {
		strs := make([]string, 0, len(list))
		allStrings := true
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				allStrings = false
				break
			}
			strs = append(strs, s)
		}
		if allStrings {
			sort.Strings(strs)
			encoded, err := json.Marshal(strs)
			return string(encoded), err
		}
	}

	encoded, err := json.Marshal(value)
	return string(encoded), err
}

// testCheckSettingJSON asserts that a single index setting has the expected
// value on the Meilisearch server.
func testCheckSettingJSON(uid, field, want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		settings, err := indexSettings(uid)
		if err != nil {
			return err
		}

		got, err := canonicalJSON(settings[field])
		if err != nil {
			return err
		}

		var wantValue interface{}
		if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
			return fmt.Errorf("invalid expected JSON %q: %w", want, err)
		}
		wantCanonical, err := canonicalJSON(wantValue)
		if err != nil {
			return err
		}

		if got != wantCanonical {
			return fmt.Errorf("index %q setting %q: got %s, want %s", uid, field, got, wantCanonical)
		}

		return nil
	}
}

// testCheckSettingAbsent asserts that a keyed setting (such as embedders) no
// longer contains the given key on the server.
func testCheckSettingAbsent(uid, field, key string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		settings, err := indexSettings(uid)
		if err != nil {
			return err
		}

		entries, ok := settings[field].(map[string]interface{})
		if !ok {
			return nil
		}

		if _, found := entries[key]; found {
			return fmt.Errorf("index %q setting %q still contains key %q", uid, field, key)
		}

		return nil
	}
}

// setSettingOutOfBand writes a setting directly, simulating a change made
// outside Terraform, and waits for the resulting task to finish.
func setSettingOutOfBand(uid, settingPath, body string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		data, err := meilisearchRequest(http.MethodPut, "/indexes/"+uid+"/settings/"+settingPath, body)
		if err != nil {
			return err
		}

		var task struct {
			TaskUID int64 `json:"taskUid"`
		}
		if err := json.Unmarshal(data, &task); err != nil {
			return err
		}

		return waitForTestTask(task.TaskUID)
	}
}

func waitForTestTask(taskUID int64) error {
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		data, err := meilisearchRequest(http.MethodGet, fmt.Sprintf("/tasks/%d", taskUID), "")
		if err != nil {
			return err
		}

		var task struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(data, &task); err != nil {
			return err
		}

		switch task.Status {
		case "succeeded":
			return nil
		case "failed", "canceled":
			return fmt.Errorf("task %d finished with status %q: %s", taskUID, task.Status, data)
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for task %d", taskUID)
}
