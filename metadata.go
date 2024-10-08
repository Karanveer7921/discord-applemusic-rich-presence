package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/log"
	"github.com/cheshir/ttlcache"
)

type SongMetadata struct {
	ID           string
	AlbumArtwork string
	ShareURL     string
}

const baseURL = "https://itunes.apple.com/search"

func get(url string, result interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bts, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bts))
	}

	if err := json.Unmarshal(bts, result); err != nil {
		return fmt.Errorf("%w: %s", err, string(bts))
	}

	return nil
}

func getSongMetadata(key string) (SongMetadata, error) {
	var result getSongMetadataResult
	err := get(baseURL+"?types=songs&limit=1&term="+key, &result)
	if err != nil {
		return SongMetadata{}, err
	}

	if len(result.Songs.Data) == 0 {
		return SongMetadata{}, nil
	}

	id := result.Songs.Data[0].ID
	artwork := result.Songs.Data[0].Attributes.Artwork.URL
	artwork = strings.Replace(artwork, "{w}", "512", 1)
	artwork = strings.Replace(artwork, "{h}", "512", 1)

	return SongMetadata{
		ID:           id,
		AlbumArtwork: artwork,
		ShareURL:     result.Songs.Data[0].Attributes.URL,
	}, nil
}

func getArtistArtwork(key string) (string, error) {
	var result getArtistMetadataResult
	err := get(baseURL+"?types=artists&limit=1&term="+key, &result)
	if err != nil {
		return "", err
	}

	if len(result.Artists.Data) == 0 {
		return "", nil
	}

	artwork := result.Artists.Data[0].Attributes.Artwork.URL
	artwork = strings.Replace(artwork, "{w}", "512", 1)
	artwork = strings.Replace(artwork, "{h}", "512", 1)

	return artwork, nil
}

func getMetadata(artist, album, song string) (Metadata, error) {
	key := url.QueryEscape(strings.Join([]string{artist, album, song}, " "))

	albumArtworkCached, albumArtworkOk := cache.albumArtwork.Get(ttlcache.StringKey(key))
	shareURLCached, shareURLOk := cache.shareURL.Get(ttlcache.StringKey(key))

	artistArtworkCached, artistArtworkOk := cache.artistArtwork.Get(ttlcache.StringKey(artist))

	if albumArtworkOk && artistArtworkOk && shareURLOk {
		log.WithField("key", key).Debug("got song info from cache")
		return Metadata{
			AlbumArtwork:  albumArtworkCached.(string),
			ArtistArtwork: artistArtworkCached.(string),
			ShareURL:      shareURLCached.(string),
		}, nil
	}

	var err error
	var songMetadata SongMetadata
	var artistArtwork string

	if albumArtworkOk && shareURLOk {
		songMetadata = SongMetadata{
			AlbumArtwork: albumArtworkCached.(string),
			ShareURL:     shareURLCached.(string),
		}
	} else {
		log.WithField("song", song).Debug("getting song metadata from api")
		songMetadata, err = getSongMetadata(key)
		if err != nil {
			return Metadata{}, err
		}
	}

	if artistArtworkOk {
		artistArtwork = artistArtworkCached.(string)
	} else {
		log.WithField("artist", artist).Debug("getting artist artwork from api")
		artistArtwork, _ = getArtistArtwork(url.QueryEscape(artist))
	}

	cache.albumArtwork.Set(ttlcache.StringKey(key), songMetadata.AlbumArtwork, time.Hour)
	cache.shareURL.Set(ttlcache.StringKey(key), songMetadata.ShareURL, time.Hour)

	cache.artistArtwork.Set(ttlcache.StringKey(artist), artistArtwork, time.Hour)

	return Metadata{
		ID:            songMetadata.ID,
		AlbumArtwork:  songMetadata.AlbumArtwork,
		ShareURL:      songMetadata.ShareURL,
		ArtistArtwork: artistArtwork,
	}, nil
}

type getSongMetadataResult struct {
	Songs struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				URL     string `json:"url"`
				Artwork struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	} `json:"songs"`
}

type getArtistMetadataResult struct {
	Artists struct {
		Data []struct {
			Attributes struct {
				Artwork struct {
					URL string `json:"url"`
				} `json:"artwork"`
			} `json:"attributes"`
		} `json:"data"`
	} `json:"artists"`
}

type Metadata struct {
	ID            string
	AlbumArtwork  string
	ArtistArtwork string
	ShareURL      string
}
