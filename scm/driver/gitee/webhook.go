// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitee

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/drone/go-scm/scm"
	"github.com/drone/go-scm/scm/driver/internal/null"
)

type webhookService struct {
	client *wrapper
}

func (s *webhookService) Parse(req *http.Request, fn scm.SecretFunc) (scm.Webhook, error) {

	data, err := ioutil.ReadAll(
		io.LimitReader(req.Body, 10000000),
	)
	if err != nil {
		return nil, err
	}

	var hook scm.Webhook
	switch req.Header.Get("X-Gitee-Event") {
	case "Push Hook": // commit push, branch push
		hook, err = s.parsePushHook(data)
	case "Tag Push Hook":
		hook, err = s.parseCreateHook(data)
	case "Merge Request Hook":
		hook, err = s.parsePullRequestHook(data)
	// case "deployment":
	// 	hook, err = s.parseDeploymentHook(data)
	// case "pull_request_review_comment":
	// case "issues":
	// case "issue_comment":
	default:
		return nil, scm.ErrUnknownEvent
	}
	if err != nil {
		return nil, err
	}

	// get the gitlab shared token to verify the payload
	// authenticity. If no key is provided, no validation
	// is performed.
	token, err := fn(hook)
	if err != nil {
		return hook, err
	} else if token == "" {
		return hook, nil
	}

	if token != req.Header.Get("X-Gitee-Token") {
		return hook, scm.ErrSignatureInvalid
	}

	return hook, nil
}

func (s *webhookService) parsePushHook(data []byte) (scm.Webhook, error) {
	dst := new(pushHook)
	err := json.Unmarshal(data, dst)
	return convertPushHook(dst), err
}

func (s *webhookService) parseCreateHook(data []byte) (scm.Webhook, error) {
	src := new(createDeleteHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertTagHook(src)
	dst.Action = scm.ActionCreate
	return dst, nil
}

func (s *webhookService) parseDeleteHook(data []byte) (scm.Webhook, error) {
	src := new(createDeleteHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	if src.RefType == "branch" {
		dst := convertBranchHook(src)
		dst.Action = scm.ActionDelete
		return dst, nil
	}
	dst := convertTagHook(src)
	dst.Action = scm.ActionDelete
	return dst, nil
}

func (s *webhookService) parsePullRequestHook(data []byte) (scm.Webhook, error) {
	src := new(pullRequestHook)
	err := json.Unmarshal(data, src)
	if err != nil {
		return nil, err
	}
	dst := convertPullRequestHook(src)
	switch src.Action {
	case "assign", "test", "approved", "tested", "merge":
		return nil, nil
	case "open":
		dst.Action = scm.ActionOpen
	case "close":
		// if merged == true
		//    dst.Action = scm.ActionMerge
		dst.Action = scm.ActionClose
	}
	return dst, nil
}

//
// native data structures
//

type (
	// github create webhook payload
	createDeleteHook struct {
		Ref        string     `json:"ref"`
		RefType    string     `json:"ref_type"`
		Repository repository `json:"repository"`
		Sender     user       `json:"sender"`
	}

	// github push webhook payload
	pushHook struct {
		Ref     string `json:"ref"`
		BaseRef string `json:"base_ref"`
		Before  string `json:"before"`
		After   string `json:"after"`
		Compare string `json:"compare"`
		Head    struct {
			ID        string    `json:"id"`
			TreeID    string    `json:"tree_id"`
			Distinct  bool      `json:"distinct"`
			Message   string    `json:"message"`
			Timestamp null.Time `json:"timestamp"`
			URL       string    `json:"url"`
			Author    struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"author"`
			Committer struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"committer"`
			Added    []interface{} `json:"added"`
			Removed  []interface{} `json:"removed"`
			Modified []string      `json:"modified"`
		} `json:"head_commit"`
		Commits []struct {
			ID        string    `json:"id"`
			TreeID    string    `json:"tree_id"`
			Distinct  bool      `json:"distinct"`
			Message   string    `json:"message"`
			Timestamp null.Time `json:"timestamp"`
			URL       string    `json:"url"`
			Author    struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"author"`
			Committer struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			} `json:"committer"`
			Added    []interface{} `json:"added"`
			Removed  []interface{} `json:"removed"`
			Modified []string      `json:"modified"`
		} `json:"commits"`
		Repository struct {
			ID    int64 `json:"id"`
			Owner struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
			} `json:"owner"`
			Name          string `json:"name"`
			FullName      string `json:"full_name"`
			Private       bool   `json:"private"`
			Fork          bool   `json:"fork"`
			HTMLURL       string `json:"html_url"`
			SSHURL        string `json:"ssh_url"`
			CloneURL      string `json:"clone_url"`
			DefaultBranch string `json:"default_branch"`
		} `json:"repository"`
		Pusher user `json:"pusher"`
		Sender user `json:"sender"`
	}

	pullRequestHook struct {
		Action      string     `json:"action"` // 'assign' -> 'test' -> 'open' -> ('close'->|'approved') -> 'tested' -> 'merge' -> 'master commit push(not pr event is push event)'
		Number      int        `json:"number"`
		PullRequest pr         `json:"pull_request"`
		Repository  repository `json:"repository"`
		Sender      user       `json:"sender"`
	}
)

//
// native data structure conversion
//

func convertPushHook(src *pushHook) *scm.PushHook {
	var commits []scm.Commit
	for _, c := range src.Commits {
		commits = append(commits,
			scm.Commit{
				Sha:     c.ID,
				Message: c.Message,
				Link:    c.URL,
				Author: scm.Signature{
					Login: c.Author.Username,
					Email: c.Author.Email,
					Name:  c.Author.Name,
					Date:  c.Timestamp.ValueOrZero(),
				},
				Committer: scm.Signature{
					Login: c.Committer.Username,
					Email: c.Committer.Email,
					Name:  c.Committer.Name,
					Date:  c.Timestamp.ValueOrZero(),
				},
			})
	}
	dst := &scm.PushHook{
		Ref:     src.Ref,
		BaseRef: src.BaseRef,
		Before:  src.Before,
		After:   src.After,
		Commit: scm.Commit{
			Sha:     src.After,
			Message: src.Head.Message,
			Link:    src.Compare,
			Author: scm.Signature{
				Login: src.Head.Author.Username,
				Email: src.Head.Author.Email,
				Name:  src.Head.Author.Name,
				Date:  src.Head.Timestamp.ValueOrZero(),
			},
			Committer: scm.Signature{
				Login: src.Head.Committer.Username,
				Email: src.Head.Committer.Email,
				Name:  src.Head.Committer.Name,
				Date:  src.Head.Timestamp.ValueOrZero(),
			},
		},
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender:  *convertUser(&src.Sender),
		Commits: commits,
	}
	// fix https://github.com/drone/go-scm/issues/8
	if scm.IsTag(dst.Ref) && src.Head.ID != "" {
		dst.Commit.Sha = src.Head.ID
		dst.After = src.Head.ID
	}
	return dst
}

func convertBranchHook(src *createDeleteHook) *scm.BranchHook {
	return &scm.BranchHook{
		Ref: scm.Reference{
			Name: src.Ref,
		},
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender: *convertUser(&src.Sender),
	}
}

func convertTagHook(src *createDeleteHook) *scm.TagHook {
	return &scm.TagHook{
		Ref: scm.Reference{
			Name: src.Ref,
		},
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		Sender: *convertUser(&src.Sender),
	}
}

func convertPullRequestHook(src *pullRequestHook) *scm.PullRequestHook {
	return &scm.PullRequestHook{
		// Action        Action
		Repo: scm.Repository{
			ID:        fmt.Sprint(src.Repository.ID),
			Namespace: src.Repository.Owner.Login,
			Name:      src.Repository.Name,
			Branch:    src.Repository.DefaultBranch,
			Private:   src.Repository.Private,
			Clone:     src.Repository.CloneURL,
			CloneSSH:  src.Repository.SSHURL,
			Link:      src.Repository.HTMLURL,
		},
		PullRequest: *convertPullRequest(&src.PullRequest),
		Sender:      *convertUser(&src.Sender),
	}
}

// regexp help determine if the named git object is a tag.
// this is not meant to be 100% accurate.
var tagRE = regexp.MustCompile("^v?(\\d+).(.+)")
