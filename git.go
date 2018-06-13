package git

import (
	"fmt"
	"forjj/utils"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/forj-oss/forjj-modules/trace"
)

// Git Call git command with arguments. All print out displayed. It returns git Return code.
func Do(opts ...string) int {
	colorCyan, colorReset := utils.DefColor(36)
	log.Printf("%sgit %s%s\n", colorCyan, strings.Join(opts, " "), colorReset)
	return utils.RunCmd("git", opts...)
}

// Status return an GitStatus struct with the list of files, added, updated and
func GetStatus() (gs *Status) {
	gs = new(Status)

	gs.Ready = make(map[string][]string)
	gs.Ready.init(false)
	gs.NotReady = make(map[string][]string)
	gs.NotReady.init(true)

	ReadyRE, _ := regexp.Compile("^([ADM])  (.*)$")
	NotReadyRE, _ := regexp.Compile("^ ([?ADM]) (.*)$")

	var s string

	s, gs.Err = Get("status", "--porcelain")
	if gs.Err != nil || s == "" {
		return
	}

	lines := strings.Split(s, "\n")

	for _, line := range lines {
		if m := ReadyRE.FindStringSubmatch(line); m != nil {
			gs.Ready.add(m[1], m[2])
		}
		if m := NotReadyRE.FindStringSubmatch(line); m != nil {
			gs.Ready.add(m[1], m[2])
		}
	}
	return
}

// Get Call a git command and get the output as string output.
func Get(opts ...string) (string, error) {
	gotrace.Trace("RUNNING: git %s", strings.Join(opts, " "))
	out, err := exec.Command("git", opts...).Output()
	return string(out), err
}

// GetWithStatusCode Call a git command and get the output as string output.
func GetWithStatusCode(opts ...string) (string, int) {
	colorCyan, colorReset := utils.DefColor(36)
	log.Printf("%sgit %s%s\n", colorCyan, strings.Join(opts, " "), colorReset)
	return utils.RunCmdOutput("git", opts...)
}

// Commit Do a git commit
func Commit(msg string, errorIfEmpty bool) (err error) {
	s := GetStatus()
	if s.Ready.CountTracked() == 0 {
		if errorIfEmpty {
			err = fmt.Errorf("No files to commit. Please check")
		}
		return
	}
	if Do("commit", "-m", msg) > 0 {
		return fmt.Errorf("Unable to commit")
	}
	return nil
}

// Push Push latest commits
func Push() error {
	if Do("push") > 0 {
		return fmt.Errorf("Unable to push commits.")
	}
	return nil
}

func Add(files []string) int {
	cmd := make([]string, 1, len(files)+1)
	cmd[0] = "add"
	cmd = append(cmd, files...)
	return Do(cmd...)
}

func Branches() ([]string, error) {
	v, err := Get("branch")
	if err != nil || v == "" {
		return []string{}, err
	}
	return strings.Split(v, "\n"), nil
}

// RemoteBranches returns the list of Remote branches found
// Formatted as <remote>/<branchName>
func RemoteBranches() ([]string, error) {
	v, err := Get("branch", "-r")
	if err != nil || v == "" {
		return []string{}, err
	}
	return strings.Split(v, "\n"), nil
}

// RemoteBranchExist check is remote branch if known by GIT.
//
// Remote: Formated as <remote>/<branchName>
func RemoteBranchExist(remote string) (bool, error) {
	if branches, err := RemoteBranches(); err != nil {
		return false, err
	} else {
		for _, branch := range branches {
			if branch == remote {
				return true, nil
			}
		}
	}
	return false, nil
}

func BranchExist(remote string) (bool, error) {
	if branches, err := Branches(); err != nil {
		return false, err
	} else {
		for _, branch := range branches {
			if branch == remote {
				return true, nil
			}
		}
	}
	return false, nil
}

func RemoteStatus(remote string) (string, error) {
	var local_rev, remote_rev, base_rev string
	if v, err := Get("rev-parse", "@{0}"); err != nil {
		return "", err
	} else {
		local_rev = v
	}
	if v, err := Get("rev-parse", remote); err != nil {
		return "", err
	} else {
		remote_rev = v
	}
	if v, err := Get("merge-base", "@{0}", remote); err != nil {
		return "", err
	} else {
		base_rev = v
	}

	if local_rev == remote_rev {
		return "=", nil
	}
	if local_rev == base_rev {
		return "-1", nil
	}
	if remote_rev == base_rev {
		return "+1", nil
	}
	return "-1+1", nil
}

// RemoteExist return true if remote is defined.
func RemoteExist(remote string) (found bool) {
	var remotes []string
	if v, err := Get("remote"); err != nil {
		return
	} else {
		remotes = strings.Split(v, "\n")
	}

	for _, aRemote := range remotes {
		if aRemote == remote {
			return true
		}
	}
	return
}

func RemoteUrl(remote string) (string, bool, error) {
	var remotes []string
	if v, err := Get("remote", "-v"); err != nil {
		return "", false, err
	} else {
		if v == "" {
			remotes = []string{}
		} else {
			remotes = strings.Split(v, "\n")
		}
	}

	remMatch, _ := regexp.Compile(`^ *(\w+) *(.*) \((fetch|push)\)$`)
	for _, aRemote := range remotes {
		if v := remMatch.FindStringSubmatch(aRemote); v[0] == remote {
			return v[1], true, nil
		}
	}
	return "", false, nil
}

func EnsureRemoteIs(name, url string) error {
	if ru, found, err := RemoteUrl(name); err != nil {
		return err
	} else if found {
		if ru != url {
			Do("remote", "set-url", url)
		}
	} else {
		Do("remote", "add", name, url)
	}
	return nil
}

// GetCurrentBranch return the current branch name.
// If no branch is detected, it returns "master"
func GetCurrentBranch() (branch string) {
	if b, status := GetWithStatusCode("rev-parse", "--abbrev-ref", "HEAD"); status == 128 {
		return "master"
	} else {
		branch = b
	}
	return
}

// EnsureRepoExist ensure a local repo exist.
func EnsureRepoExist(aPath string) error {
	if fi, err := os.Stat(path.Join(aPath, ".git")); err != nil && os.IsNotExist(err) {
		if Do("init", aPath) != 0 {
			return fmt.Errorf("Unable to create the local repository '%s'", aPath)
		}
	} else if err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("'%s' is not a valid GIT repo (.git is not a directory)", aPath)
	}
	return nil
}
