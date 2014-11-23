package main

import (
    "log"
    "time"
    "encoding/json"
    "net/url"
    "net/http"
    "regexp"
    "strings"

    "github.com/garyburd/redigo/redis"
    "github.com/PuerkitoBio/goquery"
)

type Item struct {
    Price string
    Link string
}

var (
    TITLE_CLEAN_REGEX = regexp.MustCompile("[^a-zA-Z\\d\\s:]")
    PRICE_REGEX = regexp.MustCompile("(\\$[0-9]+(\\.[0-9]{2})?)")
)

func cleanTitle(title string) string {
    newTitle := TITLE_CLEAN_REGEX.ReplaceAll([]byte(title), []byte(""))
    return string(newTitle)
}

func yo(rClient redis.Conn, yoUsername string, link string) {
    API_URL := "https://api.justyo.co/yo/"
    API_TOKEN := "c6e04aa80b8ffdb1326648aeaecb81ecd0f05aed"

    key := yoUsername + ":" + link
    yoed, _ := rClient.Do("GET", key)

    if yoed == nil {
        // So we don't repeat yoing people.
        rClient.Do("SET", key, 1)

        http.PostForm(API_URL,
            url.Values{
                "api_token": {API_TOKEN},
                "link": {"http://www.bestbuy.com" + link},
                "username": {yoUsername},
            },
        )
    }
}

func scrapeBestBuy(rPool *redis.Pool, wishList []*Wish) {
    rClient := rPool.Get()
    defer rClient.Close()

    URL := "http://www.bestbuy.com/site/misc/black-friday/pcmcat225600050002.c"
    doc, err := goquery.NewDocument(URL)

    if err != nil {
        log.Fatal("BestBuy scraper failed:", err.Error())
    }

    searchMap := make(map[string]*Item)
    doc.Find(".feature-module").Each(func(i int, s *goquery.Selection) {
        title := strings.Split(
            cleanTitle(s.Find("h4").Find("a").Text()), " ")
        link, linkExists := s.Find(".sku-title a").Attr("href")
        price := PRICE_REGEX.FindString(s.Find(".item-price").Text())

        if linkExists {
            for _, keyword := range title {
                searchMap[strings.ToLower(strings.TrimSpace(keyword))] = &Item{
                    price,
                    link,
                }
            }
        }
    })

    // Do the most stupid thing ever :(
    for _, wish := range wishList {
        if wish == nil {
            continue
        }

        keywords := strings.Split(wish.ProductKeywords, ",")
        for _, keyword := range keywords {
            keyword = strings.ToLower(strings.TrimSpace(keyword))
            if item, exists := searchMap[keyword]; exists {
                yo(rClient, wish.YoUsername, item.Link)
            }
        }
    }
}

const (
    SCRAPE_INTERVAL = 30 * time.Second
)

var (
    scrapers = []func(*redis.Pool, []*Wish){
        scrapeBestBuy,
    }
)

func goScraper(rPool *redis.Pool, quitChan chan int) {
    ticker := time.NewTicker(SCRAPE_INTERVAL)

    for {
        select {
            case <- ticker.C: {
                rClient := rPool.Get()
                // Fetch the list
                wishJsonList, err := redis.Strings(rClient.Do("LRANGE", CUSTOMER_QUEUE_KEY, 0, -1))

                if err != nil {
                    log.Println("GoScraper failed to get from redis")
                } else {
                    var (
                        err error
                    )

                    wish := &Wish{}
                    wishList := make([]*Wish, len(wishJsonList), len(wishJsonList))

                    for _, wishJson := range wishJsonList {
                        if err = json.Unmarshal([]byte(wishJson), wish); err != nil {
                            continue
                        }
                        wishList = append(wishList, wish)
                    }

                    for _, scrapeFunc := range scrapers {
                        // WOW!
                        go scrapeFunc(rPool, wishList)
                    }
                }
            }
            case <- quitChan: {
                ticker.Stop()
                return
            }
        }
    }
}
