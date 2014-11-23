package main

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "math/rand"
    "log"

    "github.com/garyburd/redigo/redis"
)

func randInt(min int, max int) int {
    return min + rand.Intn(max - min)
}

func randomString(length int) string {
    bytes := make([]byte, length)
    for i:= 0; i < length; i++ {
        bytes[i] = byte(randInt(65, 90))
    }
    return string(bytes)
}

// Get the yo username
func WishGET(w http.ResponseWriter, r *http.Request, rPool *redis.Pool) error {
    log.Println("Get Request")
    rClient := rPool.Get()
    defer rClient.Close()

    query := r.URL.Query()

    if yoUsername := query.Get("yoUsername"); yoUsername != "" {
        jsonBlob, err := redis.String(rClient.Do("GET", yoUsername))

        if err == redis.ErrNil {
            w.WriteHeader(http.StatusNotFound)
            w.Write([]byte("Not found"))
        } else if err != nil {
            return &InternalError {
                "Redis Call Failed: GET " + err.Error(),
            }
        } else {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte(jsonBlob))
            return nil
        }
    }

    return nil
}

// Create a new wishlist
func WishPOST(w http.ResponseWriter, r *http.Request, rPool *redis.Pool) error {
    log.Println("POST Request")
    rClient := rPool.Get()
    defer rClient.Close()

    wish := &Wish{}

    var (
        body []byte
        err error
    )

    if body, err = ioutil.ReadAll(r.Body); err != nil {
        return err
    }

    if err = json.Unmarshal([]byte(body), wish); err != nil {
        return &InvalidRequest {
            "Bad JSON: " + err.Error(),
        }
    }

    if _, err := rClient.Do("RPUSH", CUSTOMER_QUEUE_KEY, body); err != nil {
        return &InternalError {
            "Redis Call Failed: RPUSH " + err.Error(),
        }
    }

    if _, err := rClient.Do("SET", wish.YoUsername, body); err != nil {
        return &InternalError {
            "Redis Call Failed: SET",
        }
    }

    w.WriteHeader(http.StatusCreated)
    w.Write(body)
    return nil
}

func makeHandler(rPool *redis.Pool) handler {
    requestHandler := map[string]handlerWithDB_and_Error {
        "GET"  : WishGET,
        "POST" : WishPOST,
    }
    return requestHandlerWithDB(rPool, requestHandler)
}
