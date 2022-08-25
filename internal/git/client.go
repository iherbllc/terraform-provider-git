package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
)

type Client struct {
	username string
	password string

	mtx sync.Mutex
}

func NewClient(username, password string) *Client {
	return &Client{
		username: username,
		password: password,
		mtx:      sync.Mutex{},
	}
}

type ReadRequest struct {
	Repository string
	Branch     string
	Path       string
	FileName   string
}

type ReadResponse struct {
	Exists   bool
	Contents []byte
}

func (c *Client) Read(ctx context.Context, request ReadRequest) (*ReadResponse, error) {
	r, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           request.Repository,
		Auth:          &gitHttp.BasicAuth{Username: c.username, Password: c.password},
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", request.Branch)),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}

	t, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worktree")
	}

	fileName := fmt.Sprintf("%s/%s", request.Path, request.FileName)

	_, err = t.Filesystem.Stat(fileName)
	if err == os.ErrNotExist {
		return &ReadResponse{
			Exists: false,
		}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat file")
	}

	f, err := t.Filesystem.OpenFile(fileName, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	return &ReadResponse{
		Exists:   true,
		Contents: b,
	}, nil
}

type WriteRequest struct {
	ReadRequest
	Content string
	Name    string
	Email   string
	Postfix string
}
type WriteResponse struct{}

func (c *Client) Write(ctx context.Context, request WriteRequest) (*WriteResponse, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	tries := 0
start:
	r, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           request.Repository,
		Auth:          &gitHttp.BasicAuth{Username: c.username, Password: c.password},
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", request.Branch)),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to clone repository")
	}

	t, err := r.Worktree()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get worktree")
	}

	err = t.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", request.Branch)),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to checkout")
	}

	fileName := fmt.Sprintf("%s/%s", request.Path, request.FileName)
	f, err := t.Filesystem.Create(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file")
	}
	defer f.Close()

	_, err = f.Write([]byte(request.Content))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write content")
	}
	f.Close()

	_, err = t.Add(fileName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add file")
	}

	_, err = t.Commit(fmt.Sprintf("%s: updated via terraform. %s", request.FileName, request.Postfix), &git.CommitOptions{
		Author: &object.Signature{
			Name:  request.Name,
			Email: request.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to commit")
	}

	err = r.PushContext(ctx, &git.PushOptions{
		Auth: &gitHttp.BasicAuth{Username: c.username, Password: c.password},
	})
	if err != nil && strings.Contains(err.Error(), git.ErrNonFastForwardUpdate.Error()) {
		tries++
		if tries > 3 {
			return nil, errors.Wrap(err, "failed to push. retried 3 times")
		}
		goto start
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to push")
	}

	return nil, nil
}
