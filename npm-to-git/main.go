// Command npm-to-git converts npm packages to git repositories.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func main() {
	var (
		pkgName = flag.String("package", "", "npm package name to convert to git")
		outDir  = flag.String("output", "", "output directory (default: package name)")
		allVers = flag.Bool("all-versions", false, "download all versions")
		monitor = flag.Bool("monitor", false, "monitor the package for changes")
		history = flag.String("history", ".npm-to-git-history.txt", "history file path")
	)
	flag.Parse()

	if *pkgName == "" {
		log.Fatal("Error: package name is required. Use -h for help.")
	}

	if *outDir == "" {
		*outDir = *pkgName
	}

	cfg := config{
		packageName:  *pkgName,
		outputDir:    *outDir,
		historyFile:  *history,
	}

	switch {
	case *allVers:
		log.Printf("Processing all versions of %s", *pkgName)
		if err := processAllVersions(cfg); err != nil {
			log.Fatal(err)
		}
	case *monitor:
		log.Printf("Starting monitoring mode for %s", *pkgName)
		if err := monitorPackage(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		log.Printf("Processing current version of %s", *pkgName)
		if err := processCurrentVersion(cfg); err != nil {
			log.Fatal(err)
		}
	}
}

type config struct {
	packageName  string
	outputDir    string
	historyFile  string
}

type packageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Description     string            `json:"description"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type packageVersion struct {
	Version string
	Date    time.Time
}

// processCurrentVersion processes the latest version of a package.
func processCurrentVersion(cfg config) error {
	tempDir, err := ioutil.TempDir("", "npm-to-git-")
	if err != nil {
		return fmt.Errorf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := installPackage(tempDir, cfg.packageName, ""); err != nil {
		return fmt.Errorf("install package: %v", err)
	}

	// Locate the installed package
	pkgDir := filepath.Join(tempDir, "node_modules", cfg.packageName)
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return fmt.Errorf("package directory not found at %s", pkgDir)
	}

	// Read package.json
	pkg, err := readPackageJSON(pkgDir)
	if err != nil {
		return fmt.Errorf("read package.json: %v", err)
	}

	// Create and initialize git repo
	if err := os.MkdirAll(cfg.outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %v", err)
	}

	if err := gitInit(cfg.outputDir); err != nil {
		return fmt.Errorf("initialize git repo: %v", err)
	}

	// Create upstream branch
	if err := gitCreateBranch(cfg.outputDir, "upstream"); err != nil {
		return fmt.Errorf("create upstream branch: %v", err)
	}

	// Copy package files
	if err = copyDir(pkgDir, cfg.outputDir); err != nil {
		return fmt.Errorf("copy files: %v", err)
	}

	// Commit changes
	commitMsg := fmt.Sprintf("Initial commit of %s v%s", pkg.Name, pkg.Version)
	if err := gitCommit(cfg.outputDir, commitMsg); err != nil {
		return fmt.Errorf("commit files: %v", err)
	}

	// Tag with upstream prefix
	upstreamTag := fmt.Sprintf("upstream-v%s", pkg.Version)
	if err := gitAddTag(cfg.outputDir, upstreamTag, commitMsg); err != nil {
		return fmt.Errorf("add upstream tag: %v", err)
	}

	// Add standard version tag
	stdTag := "v" + pkg.Version
	if err := gitAddTag(cfg.outputDir, stdTag, commitMsg); err != nil {
		return fmt.Errorf("add standard tag: %v", err)
	}

	// Create master branch pointing to the latest version
	if err := gitCreateBranch(cfg.outputDir, "master"); err != nil {
		return fmt.Errorf("create master branch: %v", err)
	}

	log.Printf("Successfully converted %s v%s to git repository at %s", pkg.Name, pkg.Version, cfg.outputDir)
	return nil
}

// processAllVersions downloads and analyzes all versions of a package.
func processAllVersions(cfg config) error {
	// Setup directories
	versionsDir := filepath.Join(cfg.outputDir, "versions")
	repoDir := filepath.Join(cfg.outputDir, "repo")
	
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return fmt.Errorf("create versions directory: %v", err)
	}
	
	// Setup history file
	histFile, err := os.OpenFile(cfg.historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history file: %v", err)
	}
	defer histFile.Close()
	
	fmt.Fprintf(histFile, "\n--- Version scan for %s at %s ---\n", cfg.packageName, time.Now().Format(time.RFC3339))
	
	// Get all versions
	versions, err := getVersions(cfg.packageName)
	if err != nil {
		return fmt.Errorf("get versions: %v", err)
	}
	
	log.Printf("Found %d versions of %s", len(versions), cfg.packageName)
	fmt.Fprintf(histFile, "Found %d versions\n", len(versions))
	
	// Download and install all versions
	installed, err := downloadVersions(cfg, versions, versionsDir, histFile)
	if err != nil {
		return fmt.Errorf("download versions: %v", err)
	}
	
	// Create git repo with version history
	if err := createVersionRepo(cfg, installed, repoDir, histFile); err != nil {
		return fmt.Errorf("create version repo: %v", err)
	}
	
	return nil
}

// downloadVersions downloads and extracts all package versions.
func downloadVersions(cfg config, versions []packageVersion, dir string, histFile *os.File) ([]versionInfo, error) {
	var installed []versionInfo
	
	for _, v := range versions {
		versionDir := filepath.Join(dir, v.Version)
		
		// Skip already processed versions
		if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
			log.Printf("Version %s already processed, skipping...", v.Version)
			
			// Load package.json for analysis
			pkg, err := readPackageJSON(versionDir)
			if err == nil {
				installed = append(installed, versionInfo{
					Version: v.Version,
					Path:    versionDir,
					Package: pkg,
				})
			}
			continue
		}
		
		log.Printf("Processing version %s...", v.Version)
		fmt.Fprintf(histFile, "Processing version %s\n", v.Version)
		
		// Create temp dir for npm install
		tempDir, err := ioutil.TempDir("", "npm-to-git-")
		if err != nil {
			log.Printf("Error creating temp dir for version %s: %v", v.Version, err)
			continue
		}
		
		// Install specific version
		if err := installPackage(tempDir, cfg.packageName, v.Version); err != nil {
			log.Printf("Error installing %s@%s: %v", cfg.packageName, v.Version, err)
			os.RemoveAll(tempDir)
			continue
		}
		
		// Find package dir
		pkgDir := filepath.Join(tempDir, "node_modules", cfg.packageName)
		if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
			log.Printf("Package directory not found for version %s", v.Version)
			os.RemoveAll(tempDir)
			continue
		}
		
		// Read package.json
		pkg, err := readPackageJSON(pkgDir)
		if err != nil {
			log.Printf("Error reading package.json for version %s: %v", v.Version, err)
			os.RemoveAll(tempDir)
			continue
		}
		
		// Create version directory
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			log.Printf("Error creating directory for version %s: %v", v.Version, err)
			os.RemoveAll(tempDir)
			continue
		}
		
		// Copy files
		if err = copyDir(pkgDir, versionDir); err != nil {
			log.Printf("Error copying files for version %s: %v", v.Version, err)
			os.RemoveAll(tempDir)
			continue
		}
		
		// Add to installed versions
		installed = append(installed, versionInfo{
			Version: v.Version,
			Path:    versionDir,
			Package: pkg,
		})
		
		os.RemoveAll(tempDir)
		log.Printf("Successfully processed version %s", v.Version)
	}
	
	// Sort versions semantically
	sort.Slice(installed, func(i, j int) bool {
		return semverLess(installed[i].Version, installed[j].Version)
	})
	
	return installed, nil
}

// versionInfo holds processed version information.
type versionInfo struct {
	Version string
	Path    string
	Package packageJSON
}

// createVersionRepo creates a git repo with all versions.
func createVersionRepo(cfg config, versions []versionInfo, repoDir string, histFile *os.File) error {
	if len(versions) == 0 {
		return fmt.Errorf("no versions to process")
	}
	
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("create repo directory: %v", err)
	}
	
	if err := gitInit(repoDir); err != nil {
		return fmt.Errorf("initialize git repo: %v", err)
	}
	
	// Create upstream branch with original npm content
	if err := gitCreateBranch(repoDir, "upstream"); err != nil {
		return fmt.Errorf("create upstream branch: %v", err)
	}
	
	// Create commits for each version on upstream branch
	for i, v := range versions {
		// Clear directory for clean copy (except .git)
		if i > 0 {
			if err := cleanDir(repoDir); err != nil {
				log.Printf("Error cleaning directory for version %s: %v", v.Version, err)
				continue
			}
		}
		
		// Copy files
		if err := copyDir(v.Path, repoDir); err != nil {
			log.Printf("Error copying files for version %s: %v", v.Version, err)
			continue
		}
		
		// Commit with tag
		commitMsg := fmt.Sprintf("%s@%s", cfg.packageName, v.Version)
		if err := gitCommit(repoDir, commitMsg); err != nil {
			log.Printf("Error creating commit for version %s: %v", v.Version, err)
			continue
		}
		
		// Add upstream tag
		upstreamTag := fmt.Sprintf("upstream-v%s", v.Version)
		if err := gitAddTag(repoDir, upstreamTag, commitMsg); err != nil {
			log.Printf("Error tagging version %s: %v", v.Version, err)
			continue
		}
		
		// Also add standard tag for compatibility
		stdTag := "v" + v.Version
		if err := gitAddTag(repoDir, stdTag, commitMsg); err != nil {
			log.Printf("Error adding standard tag for version %s: %v", v.Version, err)
		}
		
		log.Printf("Created commit and tags for version %s", v.Version)
	}
	
	// Create master branch that will point to the latest version
	if len(versions) > 0 {
		latestVersion := versions[len(versions)-1]
		
		// Create master branch
		if err := gitCreateBranch(repoDir, "master"); err != nil {
			log.Printf("Error creating master branch: %v", err)
		} else {
			// Copy latest version to master
			if err := cleanDir(repoDir); err != nil {
				log.Printf("Error cleaning directory for master branch: %v", err)
			} else if err := copyDir(latestVersion.Path, repoDir); err != nil {
				log.Printf("Error copying files to master branch: %v", err)
			} else {
				commitMsg := fmt.Sprintf("Latest version: %s@%s", cfg.packageName, latestVersion.Version)
				if err := gitCommit(repoDir, commitMsg); err != nil {
					log.Printf("Error committing master branch: %v", err)
				} else {
					log.Printf("Created master branch with latest version %s", latestVersion.Version)
				}
			}
		}
		
		// Switch back to upstream branch
		if err := gitCheckout(repoDir, "upstream"); err != nil {
			log.Printf("Error switching back to upstream branch: %v", err)
		}
	}
	
	// Analyze differences between versions
	if len(versions) >= 2 {
		analyzeChanges(repoDir, versions, histFile)
	}
	
	// Print information about branches and tags
	fmt.Fprintf(histFile, "\n--- Repository Structure ---\n")
	fmt.Fprintf(histFile, "Branches:\n")
	fmt.Fprintf(histFile, "- upstream: Contains original npm package files with history\n")
	fmt.Fprintf(histFile, "- master: Points to the latest version\n")
	fmt.Fprintf(histFile, "\nTags:\n")
	fmt.Fprintf(histFile, "- upstream-v*.*.* tags for each npm version\n")
	fmt.Fprintf(histFile, "- v*.*.* standard tags for each version\n")
	fmt.Fprintf(histFile, "\nUsage Example:\n")
	fmt.Fprintf(histFile, "- Create a 'fmt' branch from upstream for formatted code:\n")
	fmt.Fprintf(histFile, "  git checkout -b fmt upstream\n")
	fmt.Fprintf(histFile, "  <run formatter on files>\n")
	fmt.Fprintf(histFile, "  git add --all\n")
	fmt.Fprintf(histFile, "  git commit -m \"Apply formatting\"\n")
	
	log.Printf("All versions processed and git repository created at %s", repoDir)
	return nil
}

// analyzeChanges analyzes differences between versions.
func analyzeChanges(repoDir string, versions []versionInfo, histFile *os.File) {
	fmt.Fprintf(histFile, "\n--- Version Change Analysis ---\n")
	
	for i := 0; i < len(versions)-1; i++ {
		v1 := versions[i]
		v2 := versions[i+1]
		
		changes, err := analyzeVersionChanges(repoDir, "v"+v1.Version, "v"+v2.Version)
		if err != nil {
			log.Printf("Error analyzing changes between %s and %s: %v", v1.Version, v2.Version, err)
			continue
		}
		
		fmt.Fprintf(histFile, "\nChanges from %s to %s:\n", v1.Version, v2.Version)
		fmt.Fprintf(histFile, "  Files: %d added, %d modified, %d deleted\n", 
			changes.filesAdded, changes.filesModified, changes.filesDeleted)
		
		if len(changes.addedFuncs) > 0 {
			fmt.Fprintf(histFile, "  Functions added: %s\n", formatList(changes.addedFuncs, 5))
		}
		
		if len(changes.removedFuncs) > 0 {
			fmt.Fprintf(histFile, "  Functions removed: %s\n", formatList(changes.removedFuncs, 5))
		}
		
		if len(changes.addedClasses) > 0 {
			fmt.Fprintf(histFile, "  Classes added: %s\n", formatList(changes.addedClasses, 5))
		}
		
		if len(changes.removedClasses) > 0 {
			fmt.Fprintf(histFile, "  Classes removed: %s\n", formatList(changes.removedClasses, 5))
		}
	}
	
	// Overall summary
	fmt.Fprintf(histFile, "\n--- Overall Summary ---\n")
	fmt.Fprintf(histFile, "Total versions: %d\n", len(versions))
	fmt.Fprintf(histFile, "First version: %s\n", versions[0].Version)
	fmt.Fprintf(histFile, "Latest version: %s\n", versions[len(versions)-1].Version)
	
	// Calculate release frequency if possible
	first := getFirstDate(versions)
	last := getLastDate(versions)
	
	if !first.IsZero() && !last.IsZero() {
		freq := calcReleaseFrequency(first, last, len(versions))
		fmt.Fprintf(histFile, "Average release frequency: %s\n", freq)
	}
}

// versionChanges contains diff information between versions.
type versionChanges struct {
	filesAdded     int
	filesModified  int
	filesDeleted   int
	addedFuncs     []string
	removedFuncs   []string
	addedClasses   []string
	removedClasses []string
}

// monitorPackage continuously monitors a package for updates.
func monitorPackage(cfg config) error {
	// Setup dirs and history file
	if err := os.MkdirAll(cfg.outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %v", err)
	}
	
	histFile, err := os.OpenFile(cfg.historyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history file: %v", err)
	}
	defer histFile.Close()
	
	fmt.Fprintf(histFile, "\n--- Monitoring started for %s at %s ---\n", 
		cfg.packageName, time.Now().Format(time.RFC3339))
	
	// Get initial version
	lastVersion, err := getLatestVersion(cfg.packageName)
	if err != nil {
		return fmt.Errorf("get latest version: %v", err)
	}
	
	log.Printf("Current version: %s", lastVersion)
	fmt.Fprintf(histFile, "Current version: %s\n", lastVersion)
	
	// Check interval (every hour)
	checkInterval := 1 * time.Hour
	
	// Monitor loop
	for {
		time.Sleep(checkInterval)
		
		newVersion, err := getLatestVersion(cfg.packageName)
		if err != nil {
			log.Printf("Error checking for new version: %v", err)
			continue
		}
		
		// If version changed
		if newVersion != lastVersion {
			log.Printf("New version detected: %s -> %s", lastVersion, newVersion)
			fmt.Fprintf(histFile, "New version detected at %s: %s -> %s\n", 
				time.Now().Format(time.RFC3339), lastVersion, newVersion)
			
			// Download new version
			versionDir := filepath.Join(cfg.outputDir, newVersion)
			
			if err := downloadVersion(cfg.packageName, newVersion, versionDir); err != nil {
				log.Printf("Error downloading version %s: %v", newVersion, err)
				continue
			}
			
			lastVersion = newVersion
		}
	}
}

// downloadVersion downloads a specific package version.
func downloadVersion(packageName, version, outDir string) error {
	// Create temp dir
	tempDir, err := ioutil.TempDir("", "npm-to-git-")
	if err != nil {
		return fmt.Errorf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Install package
	if err := installPackage(tempDir, packageName, version); err != nil {
		return fmt.Errorf("install package: %v", err)
	}
	
	// Find package dir
	pkgDir := filepath.Join(tempDir, "node_modules", packageName)
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return fmt.Errorf("package directory not found")
	}
	
	// Create output dir
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %v", err)
	}
	
	// Copy files
	if err = copyDir(pkgDir, outDir); err != nil {
		return fmt.Errorf("copy files: %v", err)
	}
	
	return nil
}

// installPackage installs a package using npm.
func installPackage(dir, pkg, version string) error {
	// Initialize npm project
	cmd := exec.Command("npm", "init", "-y")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("initialize npm project: %v", err)
	}
	
	// Install package
	installPkg := pkg
	if version != "" {
		installPkg = pkg + "@" + version
	}
	
	cmd = exec.Command("npm", "install", installPkg)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install package: %v", err)
	}
	
	return nil
}

// readPackageJSON reads and parses a package.json file.
func readPackageJSON(dir string) (packageJSON, error) {
	var pkg packageJSON
	
	// Read file
	data, err := ioutil.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return pkg, fmt.Errorf("read package.json: %v", err)
	}
	
	// Parse JSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return pkg, fmt.Errorf("parse package.json: %v", err)
	}
	
	return pkg, nil
}

// gitInit initializes a git repository.
func gitInit(dir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	return cmd.Run()
}

// gitCreateBranch creates a new git branch.
func gitCreateBranch(dir, branch string) error {
	cmd := exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dir
	return cmd.Run()
}

// gitCreateOrphanBranch creates a new orphan branch.
func gitCreateOrphanBranch(dir, branch string) error {
	cmd := exec.Command("git", "checkout", "--orphan", branch)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return err
	}
	
	// Clean the index
	cmd = exec.Command("git", "rm", "-rf", "--ignore-unmatch", ".")
	cmd.Dir = dir
	return cmd.Run()
}

// gitCheckout checks out a git branch.
func gitCheckout(dir, branch string) error {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = dir
	return cmd.Run()
}

// gitCommit adds all files and creates a commit.
func gitCommit(dir, message string) error {
	// Add all files
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add: %v", err)
	}
	
	// Create commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit: %v", err)
	}
	
	return nil
}

// gitAddTag adds a tag to the current commit.
func gitAddTag(dir, tag, message string) error {
	cmd := exec.Command("git", "tag", "-a", tag, "-m", message)
	cmd.Dir = dir
	return cmd.Run()
}

// gitCommitWithTag adds all files, creates a commit, and adds a tag.
func gitCommitWithTag(dir, message, tag string) error {
	// Add all files
	cmd := exec.Command("git", "add", "--all")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add: %v", err)
	}
	
	// Create commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit: %v", err)
	}
	
	// Add tag
	cmd = exec.Command("git", "tag", "-a", tag, "-m", message)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git tag: %v", err)
	}
	
	return nil
}

// getLatestVersion returns the latest version of a package.
func getLatestVersion(pkg string) (string, error) {
	var stdout bytes.Buffer
	
	cmd := exec.Command("npm", "view", pkg, "version")
	cmd.Stdout = &stdout
	
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("npm view: %v", err)
	}
	
	return strings.TrimSpace(stdout.String()), nil
}

// getVersions gets all versions of a package with their publish dates.
func getVersions(pkg string) ([]packageVersion, error) {
	var stdout bytes.Buffer
	
	cmd := exec.Command("npm", "view", pkg, "time", "--json")
	cmd.Stdout = &stdout
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("npm view: %v", err)
	}
	
	// Parse JSON response
	var timeData map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &timeData); err != nil {
		return nil, fmt.Errorf("parse json: %v", err)
	}
	
	versions := []packageVersion{}
	
	// Extract versions and dates
	for version, timeStr := range timeData {
		// Skip metadata keys
		if version == "created" || version == "modified" {
			continue
		}
		
		// Parse publish time
		publishTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			log.Printf("Error parsing time for version %s: %v", version, err)
			continue
		}
		
		versions = append(versions, packageVersion{
			Version: version,
			Date:    publishTime,
		})
	}
	
	// Sort by publish date
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Date.Before(versions[j].Date)
	})
	
	return versions, nil
}

// analyzeVersionChanges compares two git versions.
func analyzeVersionChanges(repoDir, v1, v2 string) (versionChanges, error) {
	changes := versionChanges{}
	
	// Get changed files
	var stdout bytes.Buffer
	cmd := exec.Command("git", "diff", "--name-status", v1, v2)
	cmd.Dir = repoDir
	cmd.Stdout = &stdout
	
	if err := cmd.Run(); err != nil {
		return changes, fmt.Errorf("git diff: %v", err)
	}
	
	// Parse changes
	changedFiles := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range changedFiles {
		if line == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		
		switch parts[0] {
		case "A":
			changes.filesAdded++
		case "M":
			changes.filesModified++
		case "D":
			changes.filesDeleted++
		}
	}
	
	// Extract API from each version
	funcs1, classes1, err := extractAPI(repoDir, v1)
	if err != nil {
		return changes, fmt.Errorf("extract API from %s: %v", v1, err)
	}
	
	funcs2, classes2, err := extractAPI(repoDir, v2)
	if err != nil {
		return changes, fmt.Errorf("extract API from %s: %v", v2, err)
	}
	
	// Determine added/removed functions
	changes.addedFuncs = difference(funcs2, funcs1)
	changes.removedFuncs = difference(funcs1, funcs2)
	
	// Determine added/removed classes
	changes.addedClasses = difference(classes2, classes1)
	changes.removedClasses = difference(classes1, classes2)
	
	return changes, nil
}

// extractAPI extracts functions and classes from a git version.
func extractAPI(repoDir, version string) ([]string, []string, error) {
	var functions, classes []string
	
	// Create temp dir for extraction
	tmpDir, err := ioutil.TempDir("", "npm-to-git-api-")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Extract the version to temp dir
	cmd := exec.Command("bash", "-c", 
		fmt.Sprintf("cd %s && git archive %s | tar -x -C %s", repoDir, version, tmpDir))
	
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("extract version: %v", err)
	}
	
	// Find all JS files
	var jsFiles []string
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(path, ".js") {
			jsFiles = append(jsFiles, path)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, nil, fmt.Errorf("find JS files: %v", err)
	}
	
	// Extract functions and classes
	for _, file := range jsFiles {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		
		content := string(data)
		
		// Find functions
		funcRegex := regexp.MustCompile(`function\s+(\w+)\s*\(`)
		funcMatches := funcRegex.FindAllStringSubmatch(content, -1)
		
		for _, match := range funcMatches {
			if len(match) > 1 && match[1] != "" {
				functions = append(functions, match[1])
			}
		}
		
		// Find classes
		classRegex := regexp.MustCompile(`class\s+(\w+)`)
		classMatches := classRegex.FindAllStringSubmatch(content, -1)
		
		for _, match := range classMatches {
			if len(match) > 1 && match[1] != "" {
				classes = append(classes, match[1])
			}
		}
	}
	
	return uniqueStrings(functions), uniqueStrings(classes), nil
}

// cleanDir removes all files except .git.
func cleanDir(dir string) error {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read dir: %v", err)
	}
	
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %v", path, err)
		}
	}
	
	return nil
}

// copyDir copies a directory recursively.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip .git directory
		if strings.Contains(relPath, ".git") {
			return filepath.SkipDir
		}

		// Create destination path
		dstPath := filepath.Join(dst, relPath)

		// Handle directories
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Handle files
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(dstPath, data, info.Mode())
	})
}

// semverLess compares semantic versions.
func semverLess(v1, v2 string) bool {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	
	// Compare each part numerically
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		var num1, num2 int
		fmt.Sscanf(parts1[i], "%d", &num1)
		fmt.Sscanf(parts2[i], "%d", &num2)
		
		if num1 < num2 {
			return true
		} else if num1 > num2 {
			return false
		}
	}
	
	// If equal, shorter version is smaller
	return len(parts1) < len(parts2)
}

// difference returns items in b that aren't in a.
func difference(b, a []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}
	
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	
	return diff
}

// uniqueStrings returns a deduplicated slice of strings.
func uniqueStrings(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)
	
	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}
	
	return u
}

// formatList formats a list with a limit on items shown.
func formatList(items []string, limit int) string {
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	
	return strings.Join(items[:limit], ", ") + fmt.Sprintf(" and %d more", len(items)-limit)
}

// getFirstDate returns the date of the first version.
func getFirstDate(versions []versionInfo) time.Time {
	// NPM package.json doesn't contain dates, we'd need the original packageVersion
	// In a real implementation, we'd track the dates properly
	return time.Time{} // Placeholder
}

// getLastDate returns the date of the last version.
func getLastDate(versions []versionInfo) time.Time {
	if len(versions) == 0 {
		return time.Time{}
	}
	// Similar placeholder
	return time.Time{}
}

// calcReleaseFrequency calculates average time between releases.
func calcReleaseFrequency(first, last time.Time, count int) string {
	if count <= 1 {
		return "N/A"
	}
	
	totalDays := last.Sub(first).Hours() / 24
	avgDays := totalDays / float64(count-1)
	
	switch {
	case avgDays < 1:
		return fmt.Sprintf("%.1f hours", avgDays*24)
	case avgDays < 7:
		return fmt.Sprintf("%.1f days", avgDays)
	case avgDays < 30:
		return fmt.Sprintf("%.1f weeks", avgDays/7)
	default:
		return fmt.Sprintf("%.1f months", avgDays/30)
	}
}