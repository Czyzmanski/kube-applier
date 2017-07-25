package applylist

import (
	"github.com/box/kube-applier/git"
	"github.com/box/kube-applier/sysutil"
	"path/filepath"
	"sort"
)

// FactoryInterface allows for mocking out the functionality of Factory when testing the full process of an apply run.
type FactoryInterface interface {
	Create() ([]string, []string, []string, error)
}

// Factory handles constructing the list of files to apply and the blacklist.
type Factory struct {
	RepoPath      string
	BlacklistPath string
	WhitelistPath string
	FileSystem    sysutil.FileSystemInterface
	GitUtil       git.GitUtilInterface
}

// Create returns two alphabetically sorted lists: the list of files to apply, and the blacklist of files to skip.
func (f *Factory) Create() ([]string, []string, []string, error) {
	blacklist, err := f.createBlacklist()
	if err != nil {
		return nil, nil, nil, err
	}
	whitelist, err := f.createWhitelist()
	if err != nil {
		return nil, nil, nil, err
	}
	applyList, err := f.createApplyList(blacklist, whitelist)
	if err != nil {
		return nil, nil, nil, err
	}
	return applyList, blacklist, whitelist, nil
}

// purgeCommentsFromList iterates over the list contents and deletes comment
// lines. A comment is a line whose first non-space character is #
func (f *Factory) purgeCommentsFromList(rawList []string) []string {

	// http://stackoverflow.com/a/20551116/5771861
	i := 0
	for _, l := range rawList {
		// # is the comment line
		if len(l) > 0 && string(l[0]) != "#" {
			rawList[i] = l
			i++
		}
	}
	rv := rawList[:i]
	return rv
}

// createFilelist reads lines from the given file, converts the relative
// paths to full paths, and returns a sorted list of full paths.
func (f *Factory) createFileList(listFilePath string) ([]string, error) {
	if listFilePath == "" {
		return []string{}, nil
	}
	rawList, err := f.FileSystem.ReadLines(listFilePath)
	if err != nil {
		return nil, err
	}

	filteredList := f.purgeCommentsFromList(rawList)

	list := prependToEachPath(f.RepoPath, filteredList)
	sort.Strings(list)
	return list, nil
}

// createBlacklist reads lines from the blacklist file, converts the relative
// paths to full paths, and returns a sorted list of full paths.
func (f *Factory) createBlacklist() ([]string, error) {
	return f.createFileList(f.BlacklistPath)
}

// createWhitelist reads lines from the whitelist file, converts the relative
// paths to full paths, and returns a sorted list of full paths.
func (f *Factory) createWhitelist() ([]string, error) {
	return f.createFileList(f.WhitelistPath)
}

// createApplyList gets all files within the repo directory and returns a
// filtered and sorted list of full paths.
func (f *Factory) createApplyList(blacklist, whitelist []string) ([]string, error) {
	rawApplyListRelative, err := f.GitUtil.ListAllFiles()
	if err != nil {
		return nil, err
	}
	rawApplyList := prependToEachPath(f.RepoPath, rawApplyListRelative)
	applyList := filter(rawApplyList, blacklist, whitelist)
	sort.Strings(applyList)
	return applyList, nil
}

// shouldApplyPath returns true if file path should be applied, false otherwise.
// Conditions for skipping the file path are:
// 1. File path is not a .json or .yaml file
// 2. File path is listed in the blacklist
func shouldApplyPath(path string, blacklistMap, whitelistMap map[string]struct{}) bool {
	_, inBlacklist := blacklistMap[path]

	// If whitelist is empty, essentially there is no whitelist.
	inWhiteList := len(whitelistMap) == 0
	if !inWhiteList {
		_, inWhiteList = whitelistMap[path]
	}
	ext := filepath.Ext(path)
	return inWhiteList && !inBlacklist && (ext == ".json" || ext == ".yaml")
}

// filter iterates through the list of all files in the repo and filters it
// down to a list of those that should be applied.
func filter(rawApplyList, blacklist, whitelist []string) []string {
	blacklistMap := stringSliceToMap(blacklist)
	whitelistMap := stringSliceToMap(whitelist)

	applyList := []string{}
	for _, filePath := range rawApplyList {
		if shouldApplyPath(filePath, blacklistMap, whitelistMap) {
			applyList = append(applyList, filePath)
		}
	}
	return applyList
}
