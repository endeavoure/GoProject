package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// Настройки сервера
type Options struct {
	MaxConnections int
	Timeout        int
	Protocol       string
	Port           int
	Logging        bool
}

// Сервер с настройками
type Server struct {
	Opts 	    Options
	Responses   []int
	Index       int
	Mutex       sync.Mutex
	ReqCount	int
	LastTime	time.Time
}

// Определяем тип функции настройки
type Option func(*Options)

// Клиент с параметрами
type Client struct {
	ID 		int
	URL 	string
	ReqSent int
	Count 	map[int]int
	Mutex 	sync.Mutex
}

// Конструктор клиента
func NewClient(id int, url string) *Client {
	return &Client{
		ID:    id,
		URL:   url,
		Count: make(map[int]int),
	}
}

func newServer(opts ...Option) *Server {
	// Задаём дефолтные значения
	defaultOptions := Options{
		MaxConnections: 100,
		Timeout:        30,
		Protocol:       "http",
		Port:           8080,
		Logging:        true,
	}

	// Применяем функции настройки
	for _, opt := range opts {
		opt(&defaultOptions)
	}

	return &Server{Opts: defaultOptions}
}

// Функции настройки
func withMaxConn(maxConn int) Option {
	return func(o *Options) {
		o.MaxConnections = maxConn
	}
}

func withTimeout(timeout int) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

func withProtocol(protocol string) Option {
	return func(o *Options) {
		o.Protocol = protocol
	}
}

func withPort(port int) Option {
	return func(o *Options) {
		o.Port = port
	}
}

func withLogging(logging bool) Option {
	return func(o *Options) {
		o.Logging = logging
	}
}

// Загрузчик параметров
func loadConfig() []Option {
	var options []Option

	err := godotenv.Load("init.env")
	if err != nil {
		log.Printf("Возникла ошибка со считыванием из .env файла: %v ", err)
	}

	if val, exists := os.LookupEnv("MAX_CONNECTIONS"); exists {
		if maxConn, err := strconv.Atoi(val); err == nil {
			options = append(options, withMaxConn(maxConn))
		}
	}

	if val, exists := os.LookupEnv("TIMEOUT"); exists {
		if timeout, err := strconv.Atoi(val); err == nil {
			options = append(options, withTimeout(timeout))
		}
	}

	if val, exists := os.LookupEnv("PROTOCOL"); exists {
		options = append(options, withProtocol(val))
	}

	if val, exists := os.LookupEnv("PORT"); exists {
		if port, err := strconv.Atoi(val); err == nil {
			options = append(options, withPort(port))
		}
	}

	if val, exists := os.LookupEnv("LOGGING"); exists {
		if logging, err := strconv.ParseBool(val); err == nil {
			options = append(options, withLogging(logging))
		}
	}

	return options
}

// Генератор ответов по условию задания
func generateResp(amount int) []int {
	var responses []int

	positive := []int{http.StatusOK, http.StatusAccepted}
	negative := []int{http.StatusBadRequest, http.StatusInternalServerError}

	for i := 0; i < amount * 70/100; i++ {
		responses = append(responses, positive[rand.Intn(len(positive))])
	}
	for i := 0; i < amount * 30/100; i++ {
		responses = append(responses, negative[rand.Intn(len(negative))])
	}

	rand.Shuffle(len(responses), func(i int, j int) {responses[i], responses[j] = responses[j], responses[i]})

	return responses
}

// Отправка запросов
func (c *Client) sendPostRequest(wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 5; i++ {
		data := []byte(fmt.Sprintf(`{"message"}: "Клиент %d, Запрос %d"`, c.ID, i+1))
		resp, err := http.Post(c.URL, "application/json", bytes.NewBuffer(data))

		if err != nil {
			log.Printf("Клиент %d: Ошибка '%v' при отправке запроса.\n", c.ID, err)
			continue
		}

		// Обновляем статы
		c.Mutex.Lock()
		c.ReqSent++
		c.Count[resp.StatusCode]++
		c.Mutex.Unlock()

		log.Printf("Клиент %d: Ответ сервера %d\n", c.ID, resp.StatusCode)
		resp.Body.Close()
		time.Sleep(1000 * time.Millisecond)
	}
}

// Вывод статистики
func (c *Client) getStats() {
	fmt.Printf("Клиент %d: Отправлено запросов: %d\n", c.ID, c.ReqSent)
	c.Mutex.Lock()
	for code, count := range c.Count {
		fmt.Printf("Статус %d: %d раз(а)\n", code, count)
	}
	c.Mutex.Unlock()
}

// Запуск воркеров
func (c *Client) workerStart(wg *sync.WaitGroup, total int) {
	defer wg.Done()
	var worker sync.WaitGroup

	for i := 0; i < total / 5; i++ {
		worker.Add(2)
		go c.sendPostRequest(&worker)
		go c.sendPostRequest(&worker)
		worker.Wait()
	}

	c.getStats()
}

// Отправитель запросов
// func sendPostRequest(clientID int, url string, n int, wg *sync.WaitGroup) {
// 	defer wg.Done()
// 	for i := 0; i < n; i++ {
// 		data := []byte(fmt.Sprintf(`{"message"}: "Client %d, Request %d"`, clientID, i+1))
// 		resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))

// 		if err != nil {
// 			log.Printf("Client %d: Ошибка '%v' при отправке запроса.\n", clientID, err)
// 			continue
// 		}
// 		resp.Body.Close()
// 		log.Printf("Client %d: POST запрос номер %d прошёл успешно.\n", clientID, i+1)
// 		time.Sleep(100 * time.Millisecond)
// 	}
// }

// Обработчик запросов
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	if time.Since(s.LastTime) >= time.Second {
		s.ReqCount = 0
		s.LastTime = time.Now()
	}

	if s.ReqCount >= 5 {
		http.Error(w, "Превышен лимит запросов", http.StatusTooManyRequests)
		return
	}

	s.ReqCount++

	if s.Index >= len(s.Responses) {
		http.Error(w, "Ответы кончились", http.StatusInternalServerError)
		return
	}

	status := s.Responses[s.Index]
	s.Index++
	w.WriteHeader(status)
}

// Обработчик для проверки статуса
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Проверщик статуса сервера
func checkStatus(url string) {
	statusURL := fmt.Sprintf("%s/status", url) 
	for {
		_, err := http.Get(statusURL)
		if err != nil {
			log.Printf("Сервер недоступен. Ошибка %v.\n", err)
		} else {
			log.Print("Сервер отвечает.\n")
		}
		time.Sleep(5 * time.Second)
	}
}

// Стартер сервера
func startServer(s *Server) {
	http.HandleFunc("/", s.handleRequest)
	http.HandleFunc("/status", s.statusHandler)

	log.Printf("Сервер запущен на порту: %d\n", s.Opts.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.Opts.Port), nil))
}


func main() {
	// Загружаем параметры из .env
	serverOptions := loadConfig()

	// Создаём сервер с загруженными параметрами
	server := newServer(serverOptions...)

	// Выводим настройки сервера
	fmt.Printf("Сервер запущен с настройками: %+v\n", server.Opts)

	clientResponses1 := generateResp(100)
	clientResponses2 := generateResp(100)

	// Заводим 200 ответов (по 100 на клиент)
	server.Responses = append(clientResponses1, clientResponses2...)

	// Запускаем сервер в горутине
	go startServer(server)

	time.Sleep(1 * time.Second)

	serverURL := fmt.Sprintf("http://localhost:%d", server.Opts.Port)

	var wg sync.WaitGroup
	wg.Add(2)

	client1 := NewClient(1, serverURL)
	client2 := NewClient(2, serverURL)

	go client1.workerStart(&wg, 50)
	go client2.workerStart(&wg, 50)
	go checkStatus(serverURL)

	wg.Wait()
}