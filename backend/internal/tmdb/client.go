package tmdb

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

//go:embed tv-season-aliases.json
var tvSeasonAliasData embed.FS

type tvSeasonAlias struct {
	SeriesID        int64  `json:"seriesId"`
	SeasonNumber    int    `json:"seasonNumber"`
	SourceTitle     string `json:"sourceTitle"`
	ExpectedName    string `json:"expectedSeasonName"`
	ExpectedAirDate string `json:"expectedAirDate"`
	SourceURL       string `json:"sourceUrl"`
	Reason          string `json:"reason"`
}

var tvSeasonAliases = loadTVSeasonAliases()

func loadTVSeasonAliases() map[string]tvSeasonAlias {
	content, err := tvSeasonAliasData.ReadFile("tv-season-aliases.json")
	if err != nil {
		panic(err)
	}
	var aliases map[string]tvSeasonAlias
	if err := json.Unmarshal(content, &aliases); err != nil {
		panic(err)
	}
	return aliases
}

type Client struct {
	BaseURL   string
	APIKey    string
	Token     string
	HTTP      *http.Client
	mu        sync.Mutex
	imageBase string
}

func New(key, token string) *Client {
	return &Client{BaseURL: "https://api.themoviedb.org/3", APIKey: key, Token: token, HTTP: &http.Client{Timeout: 15 * time.Second}}
}
func (c *Client) Configured() bool { return c.APIKey != "" || c.Token != "" }

type Country struct {
	Code string `json:"iso_3166_1"`
	Name string `json:"name"`
}
type Details struct {
	ID            int64   `json:"id"`
	IMDbID        string  `json:"imdb_id"`
	IsTV          bool    `json:"-"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	ReleaseDate   string  `json:"release_date"`
	Runtime       int     `json:"runtime"`
	PosterPath    *string `json:"poster_path"`
	Genres        []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Countries []Country `json:"production_countries"`
	Credits   struct {
		Crew []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
	ImageBase string
}

type searchMovie struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
}

type movieSearchResponse struct {
	Results    []searchMovie `json:"results"`
	Page       int           `json:"page"`
	TotalPages int           `json:"total_pages"`
}

func (c *Client) request(ctx context.Context, path string, query url.Values, output any) error {
	if !c.Configured() {
		return errors.New("TMDb credentials are not configured")
	}
	if query == nil {
		query = url.Values{}
	}
	if c.APIKey != "" {
		query.Set("api_key", c.APIKey)
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path+"?"+query.Encode(), nil)
		if err != nil {
			return errors.New("invalid TMDb endpoint")
		}
		req.Header.Set("Accept", "application/json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		response, err := c.HTTP.Do(req)
		retryDelay := time.Duration(attempt+1) * 300 * time.Millisecond
		if err == nil {
			if response.StatusCode == http.StatusOK {
				body, readErr := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024+1))
				response.Body.Close()
				if readErr != nil {
					return errors.New("could not read TMDb response")
				}
				if len(body) > 4*1024*1024 {
					return errors.New("TMDb response exceeds limit")
				}
				if err := json.Unmarshal(body, output); err != nil {
					return errors.New("invalid TMDb JSON response")
				}
				return nil
			}
			io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			response.Body.Close()
			if response.StatusCode != 429 && response.StatusCode < 500 {
				return fmt.Errorf("TMDb returned HTTP %d", response.StatusCode)
			}
			if seconds, parseErr := strconv.Atoi(response.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				retryDelay = time.Duration(min(seconds, 5)) * time.Second
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt == 2 {
			return errors.New("TMDb temporarily unavailable after 3 attempts")
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return errors.New("TMDb request failed")
}

func (c *Client) ImageBase(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.imageBase != "" {
		return c.imageBase, nil
	}
	var config struct {
		Images struct {
			SecureBaseURL string   `json:"secure_base_url"`
			Sizes         []string `json:"poster_sizes"`
		} `json:"images"`
	}
	if err := c.request(ctx, "/configuration", nil, &config); err != nil {
		return "", err
	}
	parsed, err := url.Parse(config.Images.SecureBaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", errors.New("TMDb returned invalid image configuration")
	}
	found := false
	for _, size := range config.Images.Sizes {
		if size == "w500" {
			found = true
		}
	}
	if !found {
		return "", errors.New("TMDb w500 poster size unavailable")
	}
	c.imageBase = config.Images.SecureBaseURL
	return c.imageBase, nil
}

func (c *Client) Lookup(ctx context.Context, imdbID string, mapping *int64) (Details, error) {
	id := int64(0)
	if mapping != nil {
		id = *mapping
	} else {
		var found struct {
			Movies []struct {
				ID int64 `json:"id"`
			} `json:"movie_results"`
			TV []struct {
				ID int64 `json:"id"`
			} `json:"tv_results"`
		}
		if err := c.request(ctx, "/find/"+url.PathEscape(imdbID), url.Values{"external_source": {"imdb_id"}}, &found); err != nil {
			return Details{}, err
		}
		if len(found.Movies) == 1 {
			id = found.Movies[0].ID
			if len(found.TV) == 1 {
				movie, movieErr := c.LookupByTMDbID(ctx, id)
				if movieErr == nil && movie.ID == id && movie.IMDbID == imdbID {
					return movie, nil
				}
				return c.LookupByTVID(ctx, found.TV[0].ID, imdbID)
			}
		} else if len(found.Movies) > 1 {
			return Details{}, fmt.Errorf("TMDb movie match unresolved for %s (%d matches)", imdbID, len(found.Movies))
		} else if len(found.TV) == 1 {
			return c.LookupByTVID(ctx, found.TV[0].ID, imdbID)
		} else {
			if len(found.Movies) == 0 && len(found.TV) == 0 {
				return c.LookupByTVSeasonAlias(ctx, imdbID)
			}
			return Details{}, fmt.Errorf("TMDb match unresolved for %s (%d movies, %d series)", imdbID, len(found.Movies), len(found.TV))
		}
	}
	movie, err := c.LookupByTMDbID(ctx, id)
	if err != nil {
		return Details{}, err
	}
	if movie.ID != id || movie.IMDbID != imdbID {
		return Details{}, fmt.Errorf("TMDb identity mismatch for %s", imdbID)
	}
	return movie, nil
}

// LookupByTVID loads the poster and basic metadata for a series result. TMDb
// does not expose one globally unique numeric ID across its movie and TV APIs,
// so IsTV keeps that ID out of the movie-only tmdb_id column when storing.
func (c *Client) LookupByTVID(ctx context.Context, id int64, imdbID string) (Details, error) {
	if id < 1 {
		return Details{}, errors.New("invalid TMDb TV ID")
	}
	var external struct {
		IMDbID string `json:"imdb_id"`
	}
	if err := c.request(ctx, "/tv/"+strconv.FormatInt(id, 10)+"/external_ids", nil, &external); err != nil {
		return Details{}, err
	}
	if external.IMDbID != imdbID {
		return Details{}, fmt.Errorf("TMDb TV identity mismatch for %s", imdbID)
	}
	var show struct {
		ID         int64   `json:"id"`
		Name       string  `json:"name"`
		Original   string  `json:"original_name"`
		Overview   string  `json:"overview"`
		FirstAir   string  `json:"first_air_date"`
		PosterPath *string `json:"poster_path"`
		Genres     []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	if err := c.request(ctx, "/tv/"+strconv.FormatInt(id, 10), url.Values{"language": {"de-DE"}}, &show); err != nil {
		return Details{}, err
	}
	if show.ID != id || strings.TrimSpace(show.Name) == "" {
		return Details{}, errors.New("TMDb TV details identity mismatch")
	}
	if show.PosterPath != nil && (!strings.HasPrefix(*show.PosterPath, "/") || strings.Contains(*show.PosterPath, "..")) {
		return Details{}, errors.New("TMDb returned an invalid TV poster path")
	}
	base, err := c.ImageBase(ctx)
	if err != nil {
		return Details{}, err
	}
	result := Details{IMDbID: imdbID, IsTV: true, Title: show.Name, OriginalTitle: show.Original, Overview: show.Overview, ReleaseDate: show.FirstAir, PosterPath: show.PosterPath, ImageBase: base}
	for _, genre := range show.Genres {
		result.Genres = append(result.Genres, struct {
			Name string `json:"name"`
		}{Name: genre.Name})
	}
	return result, nil
}

// LookupByTVSeasonAlias resolves a reviewed IMDb-to-season match when TMDb
// exposes the work only as a season and does not return it from /find.
func (c *Client) LookupByTVSeasonAlias(ctx context.Context, imdbID string) (Details, error) {
	alias, ok := tvSeasonAliases[imdbID]
	if !ok {
		return Details{}, fmt.Errorf("TMDb match unresolved for %s (no movie or series result)", imdbID)
	}
	var season struct {
		Name       string  `json:"name"`
		Overview   string  `json:"overview"`
		AirDate    string  `json:"air_date"`
		PosterPath *string `json:"poster_path"`
	}
	path := fmt.Sprintf("/tv/%d/season/%d", alias.SeriesID, alias.SeasonNumber)
	if err := c.request(ctx, path, url.Values{"language": {"en-US"}}, &season); err != nil {
		return Details{}, err
	}
	if season.Name != alias.ExpectedName || season.AirDate != alias.ExpectedAirDate || season.PosterPath == nil {
		return Details{}, fmt.Errorf("TMDb season alias no longer matches the reviewed title for %s", imdbID)
	}
	if !strings.HasPrefix(*season.PosterPath, "/") || strings.Contains(*season.PosterPath, "..") {
		return Details{}, errors.New("TMDb returned an invalid season poster path")
	}
	base, err := c.ImageBase(ctx)
	if err != nil {
		return Details{}, err
	}
	return Details{IMDbID: imdbID, IsTV: true, Title: alias.SourceTitle, OriginalTitle: season.Name, Overview: season.Overview, ReleaseDate: season.AirDate, PosterPath: season.PosterPath, ImageBase: base}, nil
}

// LookupByTMDbID loads and validates metadata for a known TMDb identity.
func (c *Client) LookupByTMDbID(ctx context.Context, id int64) (Details, error) {
	if id < 1 {
		return Details{}, errors.New("invalid TMDb ID")
	}
	var movie Details
	if err := c.request(ctx, "/movie/"+strconv.FormatInt(id, 10), url.Values{"language": {"de-DE"}, "append_to_response": {"credits"}}, &movie); err != nil {
		return Details{}, err
	}
	if movie.ID != id || strings.TrimSpace(movie.Title) == "" {
		return Details{}, fmt.Errorf("TMDb identity mismatch for %d", id)
	}
	if movie.Runtime < 0 || movie.Runtime > 32767 {
		return Details{}, errors.New("TMDb runtime is invalid")
	}
	if movie.PosterPath != nil && (!strings.HasPrefix(*movie.PosterPath, "/") || strings.Contains(*movie.PosterPath, "..")) {
		return Details{}, errors.New("TMDb poster path is invalid")
	}
	base, err := c.ImageBase(ctx)
	if err != nil {
		return Details{}, err
	}
	movie.ImageBase = base
	return movie, nil
}

// LookupByTitle resolves only a unique exact title/year match. Ambiguous or
// fuzzy matches are returned as candidates for an explicit Cockpit correction.
func (c *Client) LookupByTitle(ctx context.Context, title string, year *int) (Details, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Details{}, errors.New("source title is required for TMDb lookup")
	}
	query := url.Values{
		"query":         {title},
		"language":      {"de-DE"},
		"include_adult": {"false"},
	}
	if year != nil {
		query.Set("year", strconv.Itoa(*year))
	}
	var response movieSearchResponse
	if err := c.request(ctx, "/search/movie", query, &response); err != nil {
		return Details{}, err
	}
	allResults := append([]searchMovie(nil), response.Results...)
	if response.TotalPages > 5 {
		return Details{}, fmt.Errorf("TMDb search for %q has more than 5 result pages; select a TMDb ID manually (examples: %s)", title, formatCandidates(nil, allResults))
	}
	for page := 2; page <= response.TotalPages; page++ {
		query.Set("page", strconv.Itoa(page))
		var additional movieSearchResponse
		if err := c.request(ctx, "/search/movie", query, &additional); err != nil {
			return Details{}, err
		}
		allResults = append(allResults, additional.Results...)
	}
	wanted := normalizeTitle(title)
	if wanted == "" {
		return Details{}, errors.New("source title has no searchable characters")
	}
	var exact []searchMovie
	for _, candidate := range allResults {
		if candidate.ID < 1 || (normalizeTitle(candidate.Title) != wanted && normalizeTitle(candidate.OriginalTitle) != wanted) {
			continue
		}
		candidateYear := releaseYear(candidate.ReleaseDate)
		if year != nil && candidateYear != *year {
			continue
		}
		exact = append(exact, candidate)
	}
	if len(exact) != 1 {
		return Details{}, fmt.Errorf("TMDb title match for %q (%s) is not unique: %s", title, yearLabel(year), formatCandidates(exact, allResults))
	}
	details, err := c.LookupByTMDbID(ctx, exact[0].ID)
	if err != nil {
		return Details{}, err
	}
	if !validIMDbID(details.IMDbID) {
		return Details{}, fmt.Errorf("TMDb result %d for %q has no usable IMDb ID", details.ID, title)
	}
	if year != nil && releaseYear(details.ReleaseDate) != *year {
		return Details{}, fmt.Errorf("TMDb year mismatch for %q: expected %d", title, *year)
	}
	return details, nil
}

func normalizeTitle(value string) string {
	var normalized strings.Builder
	for _, character := range norm.NFD.String(strings.ToLower(strings.TrimSpace(value))) {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			normalized.WriteRune(character)
		}
	}
	return normalized.String()
}

func releaseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

func validIMDbID(id string) bool {
	if len(id) < 9 || len(id) > 12 || !strings.HasPrefix(id, "tt") {
		return false
	}
	for _, digit := range id[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func yearLabel(year *int) string {
	if year == nil {
		return "ohne Jahr"
	}
	return strconv.Itoa(*year)
}

func formatCandidates(exact, all []searchMovie) string {
	candidates := exact
	if len(candidates) == 0 {
		candidates = all
	}
	if len(candidates) == 0 {
		return "keine Kandidaten; ergänze ein Jahr oder prüfe den Titel"
	}
	parts := make([]string, 0, min(5, len(candidates)))
	for _, candidate := range candidates[:min(5, len(candidates))] {
		name := candidate.Title
		if name == "" {
			name = candidate.OriginalTitle
		}
		parts = append(parts, fmt.Sprintf("TMDb %d: %q (%s)", candidate.ID, name, yearLabelFromDate(candidate.ReleaseDate)))
	}
	return strings.Join(parts, "; ")
}

func yearLabelFromDate(date string) string {
	if year := releaseYear(date); year != 0 {
		return strconv.Itoa(year)
	}
	return "Jahr unbekannt"
}
