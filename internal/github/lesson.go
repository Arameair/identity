package github

// ReadIdentityTestLesson reads one pinned, owner-authored lesson comment. It
// does not accept model-supplied URLs, issue numbers, commands, or repository names.
// Token material stays inside the original Identity device-flow client.
import (
    "context"
    "errors"
    "strings"
    "time"
)

const identityTestSource = "/repos/Arameair/Thought-Processor/issues/comments/6636766875"

func (c *Client) ReadIdentityTestLesson() (string, error) {
    c.mu.Lock()
    c.expireTokenLocked()
    if c.status != Verified || c.token == "" {
        c.mu.Unlock()
        return "", errors.New("GitHub is not connected")
    }
    token := c.token
    c.mu.Unlock()

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    var out struct {
        ID int64 `json:"id"`
        Body string `json:"body"`
        User struct { Login string `json:"login"` } `json:"user"`
    }
    code, err := c.api(ctx, token, identityTestSource, &out)
    if err != nil {
        return "", errors.New("could not contact GitHub")
    }
    if code != 200 {
        return "", errors.New("GitHub did not permit this issue-comment read")
    }
    if out.ID != 6636766875 || out.User.Login != "Arameair" ||
        !strings.Contains(out.Body, "@identity-test") ||
        len(out.Body) == 0 || len(out.Body) > 8192 {
        return "", errors.New("GitHub lesson provenance or size did not match pinned source")
    }
    c.mu.Lock()
    stillConnected := c.status == Verified && c.token == token
    c.mu.Unlock()
    if !stillConnected {
        return "", errors.New("GitHub disconnected while reading lesson")
    }
    return out.Body, nil
}
