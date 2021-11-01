package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dghubble/oauth1"
	"github.com/fogleman/gg"
)

const (
	nImages      = 17
	shadowOffset = 2
)

var parts = [][]string{
	{
		"Champ",
		"Fact:",
		"Everybody says",
		"Dang ...",
		"Check It:",
		"Just saying ...",
		"Superstar,",
		"Tiger,",
		"Self,",
		"Know this:",
		"News alert:",
		"Girl,",
		"Ace,",
		"Excuse me but",
		"Experts agree:",
		"In my opinion,",
		"Hear ye, hear ye:",
		"Okay, list up:",
	},
	{
		"the mere idea of you",
		"your soul",
		"your hair today",
		"everything you do",
		"your personal style",
		"every thought you have",
		"that sparkle in your eye",
		"your presence here",
		"what you got going on",
		"the essential you",
		"your life's journey",
		"that saucy personality",
		"your DNA",
		"that brain of yours",
		"your choice of attire",
		"the way you roll",
		"whatever your secret is",
		"all of ya'll",
	},
	{
		"has serious game",
		"rains magic",
		"deserves the Nobel Prize",
		"raises the roof",
		"breeds miracles",
		"is paying off big time",
		"shows mad skills",
		"just shimmers",
		"is a national treasure",
		"gets the party hopping",
		"is the next big thing",
		"roars like a lion",
		"is a rainbow factory",
		"is made of diamonds",
		"makes birds sing",
		"should be taught in school",
		"makes my world go 'round",
		"is 100% legit",
	},
	{
		"24/7.",
		"can I get an amen?",
		"and that's a fact.",
		"so treat yourself.",
		"you feel me?",
		"that's just science.",
		"would I lie?",
		"for reals.",
		"mic drop.",
		"you hidden gem.",
		"snuggle bear.",
		"period.",
		"now let's dance.",
		"high five.",
		"say it again!",
		"according to CNN.",
		"so get used to it.",
	},
}

var (
	twitterConfig = oauth1.NewConfig(os.Getenv("TWITTER_CONSUMER_KEY"), os.Getenv("TWITTER_CONSUMER_SECRET"))
	twitterToken  = oauth1.NewToken(os.Getenv("TWITTER_ACCESS_TOKEN"), os.Getenv("TWITTER_ACCESS_TOKEN_SECRET"))
	twitterClient = twitterConfig.Client(oauth1.NoContext, twitterToken)
)

func getRandomImage(client *s3.Client) (image.Image, error) {
	imageFileName := fmt.Sprintf("%d.jpg", rand.Int31n(nImages))
	input := &s3.GetObjectInput{
		Bucket: aws.String(os.Getenv("SOURCE_IMAGES_BUCKET")),
		Key:    aws.String(imageFileName),
	}
	resp, err := client.GetObject(context.Background(), input)
	if err != nil {
		return nil, err
	}
	return jpeg.Decode(resp.Body)
}

func generateRandomSentence() string {
	str := strings.Builder{}
	for i, strs := range parts {
		index := rand.Intn(len(strs))
		str.WriteString(strs[index])
		if i == 2 {
			str.WriteString(",")
		}
		if i < len(parts)-1 {
			str.WriteString(" ")
		}
	}
	return str.String()
}

func renderSentence(sentence string, image image.Image) (image.Image, error) {
	ctx := gg.NewContextForImage(image)
	fontSize := float64(image.Bounds().Max.Y) / 21.0
	if err := ctx.LoadFontFace("./OpenSans-SemiBold.ttf", fontSize); err != nil {
		return nil, err
	}
	x := float64(image.Bounds().Max.X) / 2.0
	y := float64(image.Bounds().Max.Y) - (fontSize * 2.0)
	ctx.SetRGB(0, 0, 0)
	ctx.DrawStringAnchored(sentence, x+shadowOffset, y+shadowOffset, 0.5, 0.5)
	ctx.SetRGB(1, 1, 1)
	ctx.DrawStringAnchored(sentence, x, y, 0.5, 0.5)
	return ctx.Image(), nil
}

type tweetMediaResponse struct {
	MediaIdString string `json:"media_id_string"`
}

func uploadImage(img image.Image) (string, error) {
	var jsonBytes bytes.Buffer
	var opts jpeg.Options
	opts.Quality = 50
	err := jpeg.Encode(&jsonBytes, img, &opts)
	if err != nil {
		return "", err
	}

	imgB64 := base64.StdEncoding.EncodeToString(jsonBytes.Bytes())

	var v url.Values = make(map[string][]string)
	v.Add("media_category", "tweet_image")
	v.Add("media_data", imgB64)

	req, err := http.NewRequest("POST", "https://upload.twitter.com/1.1/media/upload.json", bytes.NewReader([]byte(v.Encode())))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-type", "application/x-www-form-urlencoded")

	resp, err := twitterClient.Do(req)
	if err != nil {
		return "", err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("error response: %s", string(respBody))
	}

	var respStruct tweetMediaResponse
	err = json.Unmarshal(respBody, &respStruct)
	if err != nil {
		return "", err
	}

	return respStruct.MediaIdString, nil
}

func tweetMeme(text string, mediaId string) error {
	var v url.Values = make(map[string][]string)
	v.Add("status", fmt.Sprintf("\"%s\"\n - Coach Lasso (probably)", text))
	v.Add("media_ids", mediaId)
	req, err := http.NewRequest("POST", "https://api.twitter.com/1.1/statuses/update.json?"+v.Encode(), nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer: %s", os.Getenv("TWITTER_TOKEN")))
	req.Header.Add("Content-type", "application/json")

	resp, err := twitterClient.Do(req)
	if err != nil {
		return err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad response: %s", string(respBody))
	}

	log.Println(string(respBody))

	return nil
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	client := s3.NewFromConfig(cfg)

	rand.Seed(time.Now().UnixNano())
	sentence := generateRandomSentence()
	image, err := getRandomImage(client)
	if err != nil {
		panic(err)
	}
	meme, err := renderSentence(sentence, image)
	if err != nil {
		panic(err)
	}
	mediaId, err := uploadImage(meme)
	if err != nil {
		panic(err)
	}
	err = tweetMeme(sentence, mediaId)
	if err != nil {
		panic(err)
	}
}
