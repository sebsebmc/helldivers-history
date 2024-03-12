---
theme: [dark, alt]
toc: false
---

## How does this work?
By collecting data every 10 minutes and storing it in a Github repo, the git history is then traversed to build historical data used for the charts and trends.

## Why 10 minutes?
Trying to strike a balance between frequency and the amount of data we later have to process in order to generate the site.

## Its been over 10 minutes and theres no new data. What gives?
Github Actions are used to collect the data and are not guaranteed to run with a perfect schedule. Sometimes this means we don't get any data for some time. (This also helps keep the website completely free, no ads, no trackers.)

## I want to see \<X\> from the game data on this site!
Currently the data shown on this website is just what shows up in the API data that I use. Suggestion are welcome but most useful would be help with figuring out how to retreive more of the war status data (like planet effects and more order data).

## Why make this website?
I wanted an open source alternative that had as much info as possible, that was performant and also runs as hands-off as possible.

## Open Source?
The source for the API is thanks to [helldivers2-api](https://github.com/dealloc/helldivers2-api/) and the source for this website is available at [helldivers-history](https://github.com/sebsebmc/helldivers-history). We hope that the community continues to work together to help improve both.
