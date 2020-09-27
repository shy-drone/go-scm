// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitee

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/drone/go-scm/scm"

	"github.com/google/go-cmp/cmp"
)

func TestWebhooks(t *testing.T) {
	tests := []struct {
		event  string
		before string
		after  string
		obj    interface{}
	}{
		//
		// push events
		//

		// push hooks
		{
			event:  "Push Hook",
			before: "testdata/webhooks/push.json",
			after:  "testdata/webhooks/push.json.golden",
			obj:    new(scm.PushHook),
		},

		//
		// branch events
		//

		// push branch create
		{
			event:  "Push Hook",
			before: "testdata/webhooks/push_branch_create.json",
			after:  "testdata/webhooks/branch_create.json.golden",
			obj:    new(scm.BranchHook),
		},
		// push branch delete
		{
			event:  "Push Hook",
			before: "testdata/webhooks/push_branch_delete.json",
			after:  "testdata/webhooks/branch_delete.json.golden",
			obj:    new(scm.BranchHook),
		},

		//
		// tag events
		//

		// push tag create
		{
			event:  "Tag Push Hook",
			before: "testdata/webhooks/push_tag.json",
			after:  "testdata/webhooks/tag_create.json.golden",
			obj:    new(scm.TagHook),
		},
		// push tag delete
		{
			event:  "Tag Push Hook",
			before: "testdata/webhooks/push_tag_delete.json",
			after:  "testdata/webhooks/tag_delete.json.golden",
			obj:    new(scm.TagHook),
		},

		//
		// pull request events
		//

		// pull request hooks
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_opened.json",
			after:  "testdata/webhooks/pull_request_opened.json.golden",
			obj:    new(scm.PullRequestHook),
		},
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_edited.json",
			after:  "testdata/webhooks/pull_request_edited.json.golden",
			obj:    new(scm.PullRequestHook),
		},
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_synchronized.json",
			after:  "testdata/webhooks/pull_request_synchronized.json.golden",
			obj:    new(scm.PullRequestHook),
		},
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_closed.json",
			after:  "testdata/webhooks/pull_request_closed.json.golden",
			obj:    new(scm.PullRequestHook),
		},
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_reopened.json",
			after:  "testdata/webhooks/pull_request_reopened.json.golden",
			obj:    new(scm.PullRequestHook),
		},
		{
			event:  "Merge Request Hook",
			before: "testdata/webhooks/pull_request_merged.json",
			after:  "testdata/webhooks/pull_request_merged.json.golden",
			obj:    new(scm.PullRequestHook),
		},
	}

	for _, test := range tests[5:6] {
		before, err := ioutil.ReadFile(test.before)
		if err != nil {
			t.Error(err)
			continue
		}
		after, err := ioutil.ReadFile(test.after)
		if err != nil {
			t.Error(err)
			continue
		}

		buf := bytes.NewBuffer(before)
		r, _ := http.NewRequest("GET", "/", buf)
		r.Header.Set("X-Gitee-Event", test.event)
		r.Header.Set("X-Hub-Signature", "sha1=380f462cd2e160b84765144beabdad2e930a7ec5")
		r.Header.Set("X-Gitee-Delivery", "f2467dea-70d6-11e8-8955-3c83993e0aef")

		s := new(webhookService)
		o, err := s.Parse(r, secretFunc)
		if err != nil && err != scm.ErrSignatureInvalid {
			t.Error(err)
			continue
		}

		err = json.Unmarshal(after, test.obj)
		if err != nil {
			t.Error(err)
			continue
		}

		if diff := cmp.Diff(test.obj, o); diff != "" {
			t.Errorf("Error unmarshaling %s", test.before)
			t.Log(diff)

			// debug only. remove once implemented
			json.NewEncoder(os.Stdout).Encode(o)
		}

		switch event := o.(type) {
		case *scm.PushHook:
			if !strings.HasPrefix(event.Ref, "refs/") {
				t.Errorf("Push hook reference must start with refs/")
			}
		case *scm.BranchHook:
			if strings.HasPrefix(event.Ref.Name, "refs/") {
				t.Errorf("Branch hook reference must not start with refs/")
			}
		case *scm.TagHook:
			if strings.HasPrefix(event.Ref.Name, "refs/") {
				t.Errorf("Branch hook reference must not start with refs/")
			}
		}
	}
}

func TestWebhook_ErrUnknownEvent(t *testing.T) {
	f, _ := ioutil.ReadFile("testdata/webhooks/push.json")
	r, _ := http.NewRequest("GET", "/", bytes.NewBuffer(f))

	s := new(webhookService)
	_, err := s.Parse(r, secretFunc)
	if err != scm.ErrUnknownEvent {
		t.Errorf("Expect unknown event error, got %v", err)
	}
}

func TestWebhookInvalid(t *testing.T) {
	f, _ := ioutil.ReadFile("testdata/webhooks/push.json")
	r, _ := http.NewRequest("GET", "/", bytes.NewBuffer(f))
	r.Header.Set("X-Gitee-Event", "push")
	r.Header.Set("X-Gitee-Delivery", "ee8d97b4-1479-43f1-9cac-fbbd1b80da55")
	r.Header.Set("X-Hub-Signature", "sha1=380f462cd2e160b84765144beabdad2e930a7ec5")

	s := new(webhookService)
	_, err := s.Parse(r, secretFunc)
	if err != scm.ErrSignatureInvalid {
		t.Errorf("Expect invalid signature error, got %v", err)
	}
}

func TestWebhookValid(t *testing.T) {
	// the sha can be recalculated with the below command
	// openssl dgst -sha1 -hmac <secret> <file>

	f, _ := ioutil.ReadFile("testdata/webhooks/push.json")
	r, _ := http.NewRequest("GET", "/", bytes.NewBuffer(f))
	r.Header.Set("X-Gitee-Event", "push")
	r.Header.Set("X-Gitee-Delivery", "ee8d97b4-1479-43f1-9cac-fbbd1b80da55")
	r.Header.Set("X-Hub-Signature", "sha1=cf93f9ba3c8d3a789e61f91e1e5c6a360d036e98")

	s := new(webhookService)
	_, err := s.Parse(r, secretFunc)
	if err != nil {
		t.Errorf("Expect valid signature, got %v", err)
	}
}

func secretFunc(scm.Webhook) (string, error) {
	return "topsecret", nil
}
