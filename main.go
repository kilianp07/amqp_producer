package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	amqp "github.com/rabbitmq/amqp091-go"
)

var db *sql.DB
var amqpConn *amqp.Connection
var amqpChannel *amqp.Channel
var rabbitmqHost string

func init() {
	rabbitmqHost = os.Getenv("RABBITMQ_HOST")
	if rabbitmqHost == "" {
		panic("RABBITMQ_HOST environment variable is not set")
	}
	var err error
	db, err = sql.Open("sqlite3", "./data/database.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		panic(err)
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            username TEXT,
			mail TEXT,
            password TEXT
        )
    `)
	if err != nil {
		fmt.Println("Error creating table:", err)
		panic(err)
	}

	amqpConn, err = amqp.Dial("amqp://guest:guest@" + rabbitmqHost + ":5672/")
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		panic(err)
	}

	amqpChannel, err = amqpConn.Channel()
	if err != nil {
		fmt.Println("Failed to open a channel:", err)
		panic(err)
	}
}

func main() {
	r := gin.Default()

	r.POST("/api/register", registerHandler)

	err := r.Run(":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func registerHandler(c *gin.Context) {
	var user struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Mail     string `json:"mail" binding:"required"`
	}

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", user.Username, user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	// Envoyer un message AMQP après l'enregistrement de l'utilisateur
	err = amqpChannel.Publish(
		"",      // Exchange vide pour la file par défaut
		"users", // Remplacez par le nom de votre file RabbitMQ
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(user.Mail),
		},
	)
	if err != nil {
		fmt.Println("Failed to publish message to RabbitMQ:", err)
		// Vous pouvez choisir de traiter l'erreur en conséquence
	}

	c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}
