package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

func sendPostRequest(clientID int, url string, n int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < n; i++ {
		data := []byte(fmt.Sprintf(`{"message"}: "Client %d, Request %d"`, clientID, i+1))
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))

		if err != nil {
			log.Printf("Client %d: Ошибка '%v' при отправке запроса.\n", clientID, err)
			continue
		}
		resp.Body.Close()
		log.Printf("Client %d: POST запрос номер %d прошёл успешно.\n", clientID, i+1)
		time.Sleep(100 * time.Millisecond)
	}
}

func checkStatus(url string) {
	for {
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("Сервер недоступен. Ошибка %v.\n", err)
		} else {
			log.Printf("Сервер отвечает. Статус: %v.\n", resp.Status)
			resp.Body.Close()
		}
		time.Sleep(5 * time.Second)
	}
}

func startServer(port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Сервер запущен на порту:", port)
	})

	log.Println("Сервер запущен на порту:", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func main() {
	err := godotenv.Load("init.env")
	if err != nil {
		log.Println("Ошибка загрузки .env файла: ", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	go startServer(port)

	time.Sleep(2 * time.Second)
	
	serverURL := "http://localhost:" + port

	var wg sync.WaitGroup
	wg.Add(2)

	go sendPostRequest(1, serverURL, 100, &wg)
	go sendPostRequest(2, serverURL, 100, &wg)
	go checkStatus(serverURL)

	wg.Wait()
	
}