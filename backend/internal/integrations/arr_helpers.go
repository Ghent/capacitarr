package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// doRequestFunc is the function signature used by all *arr clients for GET requests.
// Each client's doRequest method matches this shape, allowing shared helper functions.
type doRequestFunc func(endpoint string) ([]byte, error)

// --- Common *arr API response types ---

// arrDiskSpace is the shared JSON shape for the diskspace endpoint across all *arr services.
type arrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

// arrRootFolder is the shared JSON shape for the root folder endpoint across all *arr services.
type arrRootFolder struct {
	Path string `json:"path"`
}

// arrQualityProfile is the shared JSON shape for quality profiles across all *arr services.
type arrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// arrTag is the shared JSON shape for tags across all *arr services.
type arrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// arrImage represents an image entry in *arr API responses.
type arrImage struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
	URL       string `json:"url"`
}

// arrExtractPosterURL finds the poster URL from an *arr images array.
// Prefers remoteUrl (external CDN) over url (local *arr path).
// Checks for coverType "poster" first, then "cover" (used by Readarr for book covers).
func arrExtractPosterURL(images []arrImage) string {
	// First pass: look for "poster" (movies, shows, artists)
	for _, img := range images {
		if img.CoverType == "poster" {
			if img.RemoteURL != "" {
				return img.RemoteURL
			}
			return img.URL
		}
	}
	// Second pass: look for "cover" (books in Readarr)
	for _, img := range images {
		if img.CoverType == "cover" {
			if img.RemoteURL != "" {
				return img.RemoteURL
			}
			return img.URL
		}
	}
	return ""
}

// --- Shared *arr fetch helpers ---

// arrFetchDiskSpace fetches and parses disk space from any *arr service.
func arrFetchDiskSpace(doReq doRequestFunc, apiPrefix string) ([]DiskSpace, error) {
	body, err := doReq(apiPrefix + "/diskspace")
	if err != nil {
		return nil, err
	}

	var result []arrDiskSpace
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse diskspace: %w", err)
	}

	disks := make([]DiskSpace, len(result))
	for i, d := range result {
		disks[i] = DiskSpace{
			Path:       d.Path,
			TotalBytes: d.TotalSpace,
			FreeBytes:  d.FreeSpace,
		}
	}
	return disks, nil
}

// arrFetchRootFolders fetches and parses root folders from any *arr service.
func arrFetchRootFolders(doReq doRequestFunc, apiPrefix string) ([]string, error) {
	body, err := doReq(apiPrefix + "/rootfolder")
	if err != nil {
		return nil, err
	}

	var folders []arrRootFolder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse root folders: %w", err)
	}

	paths := make([]string, len(folders))
	for i, f := range folders {
		paths[i] = f.Path
	}
	return paths, nil
}

// arrFetchQualityProfiles fetches quality profiles as NameValue pairs from any *arr service.
func arrFetchQualityProfiles(doReq doRequestFunc, apiPrefix string) ([]NameValue, error) {
	body, err := doReq(apiPrefix + "/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []arrQualityProfile
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

// arrFetchTags fetches tags as NameValue pairs from any *arr service.
func arrFetchTags(doReq doRequestFunc, apiPrefix string) ([]NameValue, error) {
	body, err := doReq(apiPrefix + "/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []arrTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

// arrFetchLanguages fetches languages as NameValue pairs from any *arr service.
func arrFetchLanguages(doReq doRequestFunc, apiPrefix string) ([]NameValue, error) {
	body, err := doReq(apiPrefix + "/language")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch languages: %w", err)
	}
	var langs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &langs); err != nil {
		return nil, fmt.Errorf("failed to parse languages: %w", err)
	}
	result := make([]NameValue, len(langs))
	for i, l := range langs {
		result[i] = NameValue{Value: l.Name, Label: l.Name}
	}
	return result, nil
}

// arrFetchQualityProfileMap fetches quality profiles as an ID-to-name map (used by GetMediaItems).
func arrFetchQualityProfileMap(doReq doRequestFunc, apiPrefix string) (map[int]string, error) {
	body, err := doReq(apiPrefix + "/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []arrQualityProfile
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	m := make(map[int]string, len(profiles))
	for _, p := range profiles {
		m[p.ID] = p.Name
	}
	return m, nil
}

// arrFetchTagMap fetches tags as an ID-to-label map (used by GetMediaItems).
func arrFetchTagMap(doReq doRequestFunc, apiPrefix string) (map[int]string, error) {
	body, err := doReq(apiPrefix + "/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []arrTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	m := make(map[int]string, len(tags))
	for _, t := range tags {
		m[t.ID] = t.Label
	}
	return m, nil
}

// arrResolveTagNames maps tag IDs to their label strings using a tag lookup map.
func arrResolveTagNames(tagIDs []int, tagMap map[int]string) []string {
	names := make([]string, 0, len(tagIDs))
	for _, tid := range tagIDs {
		if name, ok := tagMap[tid]; ok {
			names = append(names, name)
		}
	}
	return names
}

// arrSimpleDelete performs a simple HTTP DELETE to an *arr service endpoint.
// Used by Radarr, Lidarr, and Readarr for straightforward item deletion.
func arrSimpleDelete(baseURL, apiKey, endpoint string) error {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", apiKey)

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
