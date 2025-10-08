package truststore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/temirov/ghttp/internal/certificates"
)

const (
	firefoxUserPreferenceLine = "user_pref(\"security.enterprise_roots.enabled\", true);"
	firefoxUserPreferenceFile = "user.js"
)

func integrateFirefoxCertificates(ctx context.Context, commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration, certificatePath string) error {
	profiles := discoverFirefoxProfiles(fileSystem, configuration)
	if len(profiles) == 0 {
		return nil
	}

	certutilAvailable := isCertutilAvailable()
	var integrationErrors []error

	if certutilAvailable {
		for _, profile := range profiles {
			arguments := []string{"-A", "-n", configuration.CertificateCommonName, "-t", "C,,", "-i", certificatePath, "-d", "sql:" + profile}
			if err := commandRunner.Run(ctx, "certutil", arguments); err != nil {
				integrationErrors = append(integrationErrors, fmt.Errorf("import certificate into firefox profile %s: %w", profile, err))
			}
		}
		if len(integrationErrors) == 0 {
			return nil
		}
	}

	integrationErrors = nil
	for _, profile := range profiles {
		if err := ensureFirefoxEnterpriseRootsPreference(fileSystem, profile); err != nil {
			integrationErrors = append(integrationErrors, fmt.Errorf("enable enterprise roots for firefox profile %s: %w", profile, err))
		}
	}
	if len(integrationErrors) > 0 {
		return errors.Join(integrationErrors...)
	}
	return nil
}

func removeFirefoxCertificates(ctx context.Context, commandRunner certificates.CommandRunner, fileSystem certificates.FileSystem, configuration Configuration) error {
	profiles := discoverFirefoxProfiles(fileSystem, configuration)
	if len(profiles) == 0 {
		return nil
	}

	if !isCertutilAvailable() {
		return nil
	}

	var removalErrors []error
	for _, profile := range profiles {
		arguments := []string{"-D", "-n", configuration.CertificateCommonName, "-d", "sql:" + profile}
		if err := commandRunner.Run(ctx, "certutil", arguments); err != nil {
			removalErrors = append(removalErrors, fmt.Errorf("remove certificate from firefox profile %s: %w", profile, err))
		}
	}
	if len(removalErrors) > 0 {
		return errors.Join(removalErrors...)
	}
	return nil
}

func discoverFirefoxProfiles(fileSystem certificates.FileSystem, configuration Configuration) []string {
	candidateDirectories := configuration.FirefoxProfileDirectories
	if len(candidateDirectories) == 0 {
		candidateDirectories = defaultFirefoxProfileDirectories()
	}

	var profiles []string
	for _, directory := range candidateDirectories {
		entries, readErr := os.ReadDir(directory)
		if readErr != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			profilePath := filepath.Join(directory, entry.Name())
			if profileContainsNSSDatabase(fileSystem, profilePath) {
				profiles = append(profiles, profilePath)
			}
		}
	}
	return profiles
}

func profileContainsNSSDatabase(fileSystem certificates.FileSystem, profilePath string) bool {
	cert9Path := filepath.Join(profilePath, "cert9.db")
	exists, err := fileSystem.FileExists(cert9Path)
	if err == nil && exists {
		return true
	}
	cert8Path := filepath.Join(profilePath, "cert8.db")
	exists, err = fileSystem.FileExists(cert8Path)
	return err == nil && exists
}

func defaultFirefoxProfileDirectories() []string {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")}
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{filepath.Join(home, ".mozilla", "firefox")}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil
		}
		return []string{filepath.Join(appData, "Mozilla", "Firefox", "Profiles")}
	default:
		return nil
	}
}

func ensureFirefoxEnterpriseRootsPreference(fileSystem certificates.FileSystem, profilePath string) error {
	userPreferencesPath := filepath.Join(profilePath, firefoxUserPreferenceFile)
	existingContent, readErr := fileSystem.ReadFile(userPreferencesPath)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return readErr
	}

	contentString := string(existingContent)
	if strings.Contains(contentString, firefoxUserPreferenceLine) {
		return nil
	}
	if len(contentString) > 0 && !strings.HasSuffix(contentString, "\n") {
		contentString += "\n"
	}
	contentString += firefoxUserPreferenceLine + "\n"

	return fileSystem.WriteFile(userPreferencesPath, []byte(contentString), 0o600)
}

func isCertutilAvailable() bool {
	_, err := exec.LookPath("certutil")
	return err == nil
}
