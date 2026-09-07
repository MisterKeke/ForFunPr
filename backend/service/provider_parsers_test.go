package service

import (
	"strings"
	"testing"
)

func TestTelegramParserExtractsPostsImagesAndCanonicalIDs(t *testing.T) {
	html := `<html><body>
		<div class="tgme_widget_message_wrap" data-post="Example_Channel/1">
			<div class="tgme_widget_message_text"> Older post </div>
			<a class="tgme_widget_message_photo_wrap" style="background-image:url('https://cdn.example/one.jpg')"></a>
			<a class="tgme_widget_message_date" href="https://t.me/Example_Channel/1"><time datetime="2026-01-01T10:00:00Z"></time></a>
			<span class="tgme_widget_message_views">10K</span>
		</div>
		<div class="tgme_widget_message_wrap">
			<div class="tgme_widget_message" data-post="Example_Channel/2"></div>
			<div class="tgme_widget_message_photo"><img src="https://cdn.example/two.jpg"><img src="data:image/png;base64,ignored"></div>
			<a class="tgme_widget_message_date"><time datetime="2026-01-02T10:00:00Z"></time></a>
		</div>
	</body></html>`
	posts, err := parseTelegramPosts([]byte(html))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 || posts[0].PostID != "Example_Channel/2" || posts[1].Text != "Older post" {
		t.Fatalf("parsed posts = %#v", posts)
	}
	if len(posts[0].Images) != 1 || posts[0].Images[0] != "https://cdn.example/two.jpg" ||
		len(posts[1].Images) != 1 || posts[1].Views != "10K" {
		t.Fatalf("parsed post media = %#v", posts)
	}
	if got := telegramPostIDFromURL("https://t.me/s/example_channel/42?single#view"); got != "example_channel/42" {
		t.Fatalf("post ID from URL = %q", got)
	}
	if got := extractImageURL(`color:red;background-image: url("https://cdn.example/image.webp")`); got != "https://cdn.example/image.webp" {
		t.Fatalf("image URL = %q", got)
	}
}

func TestYouTubeChannelPageParserSupportsDocumentVariants(t *testing.T) {
	channelID := "UC0000000000000000000000"
	cases := []string{
		`<meta itemprop="channelId" content="` + channelID + `">`,
		`<link rel="canonical" href="https://www.youtube.com/channel/` + channelID + `">`,
		`<meta property="og:url" content="https://www.youtube.com/channel/` + channelID + `">`,
		`<script>window.data={"externalChannelId":"` + channelID + `"}</script>`,
	}
	for index, body := range cases {
		got, err := parseYouTubeChannelIDPage([]byte(body))
		if err != nil || got != channelID {
			t.Fatalf("variant %d = %q, %v", index, got, err)
		}
	}
	if _, err := parseYouTubeChannelIDPage([]byte(`<meta itemprop="channelId" content="invalid">`)); err == nil {
		t.Fatal("invalid channel page unexpectedly parsed")
	}
}

func TestYouTubeFeedParserSkipsInvalidEntriesAndBuildsURLs(t *testing.T) {
	channelID := "UC0000000000000000000000"
	body := `<?xml version="1.0"?>
	<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015"
		xmlns:media="http://search.yahoo.com/mrss/">
		<entry><yt:videoId>abcdefghijk</yt:videoId><title> Valid title </title>
			<published>2026-01-02T03:04:05Z</published><author><name> Channel </name></author>
			<media:group><media:thumbnail url="https://img.example/one.jpg"/><media:description> Description </media:description>
				<media:community><media:statistics views="123"/></media:community></media:group></entry>
		<entry><yt:videoId>invalid</yt:videoId><title>Ignored</title></entry>
	</feed>`
	videos, err := parseYouTubeFeed([]byte(body), channelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 || videos[0].VideoID != "abcdefghijk" || videos[0].Title != "Valid title" ||
		videos[0].VideoURL != "https://www.youtube.com/watch?v=abcdefghijk" || videos[0].Views != "123" {
		t.Fatalf("parsed videos = %#v", videos)
	}
	if _, err := parseYouTubeFeed([]byte(`<feed>`), channelID); err == nil {
		t.Fatal("malformed feed unexpectedly parsed")
	}
}

func TestSteamURLPriceAndArtworkNormalization(t *testing.T) {
	appID, canonical, err := parseSteamGameURL("https://store.steampowered.com/app/620/Portal_2/?l=english")
	if err != nil || appID != 620 || canonical != "https://store.steampowered.com/app/620/" {
		t.Fatalf("Steam URL = %d, %q, %v", appID, canonical, err)
	}
	for _, raw := range []string{
		"", "https://example.com/app/620/", "ftp://store.steampowered.com/app/620/",
		"https://user:pass@store.steampowered.com/app/620/", "https://store.steampowered.com/sub/620/",
		"https://store.steampowered.com/app/0/",
	} {
		if _, _, err := parseSteamGameURL(raw); err == nil {
			t.Fatalf("invalid Steam URL accepted: %q", raw)
		}
	}

	status, currency, regular, current, discount, err := normalizeSteamPrice(steamAppDetailsData{
		PriceOverview: &steamPriceOverview{Currency: " try ", Initial: 10000, Final: 7500, DiscountPercent: 25},
	})
	if err != nil || status != "priced" || currency != "TRY" || *regular != 10000 || *current != 7500 || discount != 25 {
		t.Fatalf("normalized Steam price = %q %q %v %v %d, %v", status, currency, regular, current, discount, err)
	}
	status, _, regular, current, _, err = normalizeSteamPrice(steamAppDetailsData{IsFree: true})
	if err != nil || status != "free" || *regular != 0 || *current != 0 {
		t.Fatalf("free Steam price = %q %v %v, %v", status, regular, current, err)
	}
	for _, price := range []*steamPriceOverview{
		{Currency: "USD", Initial: -1, Final: 0},
		{Currency: "US", Initial: 1, Final: 1},
		{Currency: "USD", Initial: 1, Final: 1, DiscountPercent: 101},
	} {
		if _, _, _, _, _, err := normalizeSteamPrice(steamAppDetailsData{PriceOverview: price}); err == nil {
			t.Fatalf("invalid Steam price accepted: %#v", price)
		}
	}

	artwork := "https://cdn.cloudflare.steamstatic.com/steam/apps/620/header.jpg?t=1"
	if got := normalizeSteamArtworkURL(artwork); got != artwork {
		t.Fatalf("artwork URL = %q", got)
	}
	if got := normalizeSteamArtworkURL("https://steamstatic.com.evil.test/image.jpg"); got != "" {
		t.Fatalf("lookalike artwork host accepted: %q", got)
	}
	if !sameSteamArtworkAsset(
		"https://cdn.cloudflare.steamstatic.com/a.jpg?one=1",
		"https://cdn.cloudflare.steamstatic.com/a.jpg?two=2",
	) {
		t.Fatal("same artwork path with cache-busting query was not recognized")
	}
	mimeType, extension, err := detectSteamGameImageFormat([]byte("\x89PNG\r\n\x1a\nmore"))
	if err != nil || mimeType != "image/png" || extension != ".png" {
		t.Fatalf("PNG detection = %q, %q, %v", mimeType, extension, err)
	}
}

func TestProviderInputNormalization(t *testing.T) {
	if got, err := NormalizeCurrencyCode(" try "); err != nil || got != "TRY" {
		t.Fatalf("currency = %q, %v", got, err)
	}
	if got, err := NormalizeTelegramUsername("@Example_Channel"); err != nil || got != "example_channel" {
		t.Fatalf("Telegram username = %q, %v", got, err)
	}
	if got, isID, err := NormalizeYouTubeReference("UC0000000000000000000000"); err != nil || !isID || !strings.HasPrefix(got, "UC") {
		t.Fatalf("YouTube channel ID = %q, %v, %v", got, isID, err)
	}
	if got, isID, err := NormalizeYouTubeReference("@Example.Handle"); err != nil || isID || got != "Example.Handle" {
		t.Fatalf("YouTube handle = %q, %v, %v", got, isID, err)
	}
}

func TestStoredWeatherValidation(t *testing.T) {
	weather := &WeatherResult{Daily: WeatherDaily{
		Time: []string{"2026-01-01"}, WeatherCode: []int{1},
		Temperature2MMax: []float64{20}, Temperature2MMin: []float64{10},
		PrecipitationProbabilityMax: []float64{30},
	}}
	payload, err := marshalWeather(weather)
	if err != nil {
		t.Fatal(err)
	}
	decoded, ok := decodeStoredWeather(payload)
	if !ok || decoded.Daily.Time[0] != "2026-01-01" {
		t.Fatalf("decoded weather = %#v, %v", decoded, ok)
	}
	if _, ok := decodeStoredWeather(`{"daily":{"time":["2026-01-01"]}}`); ok {
		t.Fatal("incomplete stored weather accepted")
	}
	for _, coordinates := range [][2]float64{{-90, -180}, {90, 180}, {0, 0}} {
		if !validWeatherCoordinates(coordinates[0], coordinates[1]) {
			t.Fatalf("valid coordinates rejected: %#v", coordinates)
		}
	}
	if validWeatherCoordinates(91, 0) || validWeatherCoordinates(0, 181) {
		t.Fatal("out-of-range weather coordinates accepted")
	}
	if got := openMeteoForecastURL(41.0082, 28.9784); got.Scheme != "https" || got.Query().Get("forecast_days") == "" {
		t.Fatalf("forecast URL = %s", got)
	}
}
