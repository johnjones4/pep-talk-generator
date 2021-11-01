build:
	GOOS=linux go build -ldflags="-s -w" -o peptalk .

deploy:
	sls deploy --verbose

push: build deploy

push-images:
	aws s3 sync ted/ s3://peptalkbotsourceimages/
