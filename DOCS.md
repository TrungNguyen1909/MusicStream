Documentation
---

# Dependencies

- You can find the required APT packages in [Aptfile](./Aptfile)

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
