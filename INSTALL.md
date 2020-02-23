Installation
---
# Containerized

- Docker, Kubernetes are supported

- Docker image at `ntrung03/musicstream`

## Docker

- Fill out the tokens in `.env` or in [docker-compose.yml](./docker-compose.yml)

- Run `docker-compose up`

- Run `docker pull ntrung03/musicstream` to update to the latest image

## Kubernetes

- Fill out the tokens in `secrets-example.yml`, base64-encoded

- Run `kubectl apply -f /path/to/secrets-example.yml` to set the secrets

- Run `kubectl apply -f k8s.yml` to start the server

- Run `kubectl rollout restart deployment/musicstream` to pull new image from docker and update the server

# Non-containerized

## Dependencies

- You can find the required APT packages in [Aptfile](./Aptfile)

## Building

- Run `go build` to build the server

## Start

- Run `./MusicStream` to start the server

- By default, the server listens at port `:8890`, but you can set that in `$PORT`

# API Tokens

Enviroment variables are also loaded from `.env` file, if exists

## Deezer

- Login to Deezer on your browser
- Find the `arl` cookies
- Put that cookie value into environment variable `DEEZER_ARL`

## Musixmatch

- Login to Musixmatch on your browser
- Find the usertoken, which is the cookies named `musixmatchUsertoken` and `OB-USER-TOKEN`
- Put their values into enviroment variables named `MUSIXMATCH_USER_TOKEN` and `MUSIXMATCH_OB_USER_TOKEN`, respectively
- The `MUSIXMATCH_OB_USER_TOKEN` is optional and can be omited if you get the usertoken from the Musixmatch's client app.
