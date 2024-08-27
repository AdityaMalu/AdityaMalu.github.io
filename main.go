package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "time"

    "github.com/go-chi/chi"
    "github.com/go-chi/chi/middleware"
    "github.com/thedevsaddam/renderer"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/mongo/readpref"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

var rnd *renderer.Render
var db *mongo.Database
var client *mongo.Client

const (
    hostName       string = "mongodb://localhost:27017"
    dbName         string = "demo_todo"
    collectionName string = "todo"
    port           string = ":9000"
)

type (
    todoModel struct {
        ID        primitive.ObjectID `bson:"_id,omitempty"`
        Title     string             `bson:"title"`
        Completed bool               `bson:"completed"`
        CreatedAt time.Time          `bson:"createdAt"`
    }

    todo struct {
        ID        string    `json:"id"`
        Title     string    `json:"title"`
        Completed bool      `json:"completed"`
        CreatedAt time.Time `json:"created_at"`
    }
)

func init() {
    rnd = renderer.New()
    clientOptions := options.Client().ApplyURI(hostName)
    var err error
    client, err = mongo.Connect(context.TODO(), clientOptions)
    checkErr(err)

    err = client.Ping(context.TODO(), readpref.Primary())
    checkErr(err)

    db = client.Database(dbName)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
    checkErr(err)
}

func createTodo(w http.ResponseWriter, r *http.Request) {
    var t todo

    if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
        rnd.JSON(w, http.StatusInternalServerError, err)
        return
    }

    if t.Title == "" {
        rnd.JSON(w, http.StatusBadRequest, renderer.M{
            "message": "The title field is required",
        })
        return
    }

    tm := todoModel{
        ID:        primitive.NewObjectID(),
        Title:     t.Title,
        Completed: false,
        CreatedAt: time.Now(),
    }
    collection := db.Collection(collectionName)
    _, err := collection.InsertOne(context.TODO(), tm)
    if err != nil {
        rnd.JSON(w, http.StatusInternalServerError, renderer.M{
            "message": "Failed to save todo",
            "error":   err,
        })
        return
    }

    rnd.JSON(w, http.StatusCreated, renderer.M{
        "message": "Todo created successfully",
        "todo_id": tm.ID.Hex(),
    })
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimSpace(chi.URLParam(r, "id"))
    objID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        rnd.JSON(w, http.StatusBadRequest, renderer.M{
            "message": "Invalid ID format",
        })
        return
    }

    var t todo

    if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
        rnd.JSON(w, http.StatusInternalServerError, err)
        return
    }

    if t.Title == "" {
        rnd.JSON(w, http.StatusBadRequest, renderer.M{
            "message": "The title field is required",
        })
        return
    }

    collection := db.Collection(collectionName)
    filter := bson.M{"_id": objID}
    update := bson.M{"$set": bson.M{"title": t.Title, "completed": t.Completed}}
    _, err = collection.UpdateOne(context.TODO(), filter, update)
    if err != nil {
        rnd.JSON(w, http.StatusInternalServerError, renderer.M{
            "message": "Failed to update todo",
            "error":   err,
        })
        return
    }

    rnd.JSON(w, http.StatusOK, renderer.M{
        "message": "Todo updated successfully",
    })
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
    collection := db.Collection(collectionName)
    cursor, err := collection.Find(context.TODO(), bson.M{})
    if err != nil {
        rnd.JSON(w, http.StatusInternalServerError, renderer.M{
            "message": "Failed to fetch todos",
            "error":   err,
        })
        return
    }
    defer cursor.Close(context.TODO())

    todos := []todoModel{}
    for cursor.Next(context.TODO()) {
        var t todoModel
        err := cursor.Decode(&t)
        if err != nil {
            rnd.JSON(w, http.StatusInternalServerError, renderer.M{
                "message": "Failed to decode todo",
                "error":   err,
            })
            return
        }
        todos = append(todos, t)
    }

    todoList := []todo{}
    for _, t := range todos {
        todoList = append(todoList, todo{
            ID:        t.ID.Hex(),
            Title:     t.Title,
            Completed: t.Completed,
            CreatedAt: t.CreatedAt,
        })
    }

    rnd.JSON(w, http.StatusOK, renderer.M{
        "data": todoList,
    })
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimSpace(chi.URLParam(r, "id"))
    objID, err := primitive.ObjectIDFromHex(id)
    if err != nil {
        rnd.JSON(w, http.StatusBadRequest, renderer.M{
            "message": "Invalid ID format",
        })
        return
    }

    collection := db.Collection(collectionName)
    filter := bson.M{"_id": objID}
    _, err = collection.DeleteOne(context.TODO(), filter)
    if err != nil {
        rnd.JSON(w, http.StatusInternalServerError, renderer.M{
            "message": "Failed to delete todo",
            "error":   err,
        })
        return
    }

    rnd.JSON(w, http.StatusOK, renderer.M{
        "message": "Todo deleted successfully",
    })
}

func main() {
    stopChan := make(chan os.Signal, 1)
    signal.Notify(stopChan, os.Interrupt)

    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Get("/", homeHandler)

    r.Mount("/todo", todoHandlers())

    srv := &http.Server{
        Addr:         port,
        Handler:      r,
        ReadTimeout:  60 * time.Second,
        WriteTimeout: 60 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        log.Println("Listening on port ", port)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %s\n", err)
        }
    }()

    <-stopChan
    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Server Shutdown Failed:%+v", err)
    }

    // Discnnect MongoDB
    if err := client.Disconnect(ctx); err != nil {
        log.Fatalf("MongoDB Disconnect Failed:%+v", err)
    }

    log.Println("Server gracefully stopped!")
}

func todoHandlers() http.Handler {
    rg := chi.NewRouter()
    rg.Group(func(r chi.Router) {
        r.Get("/", fetchTodos)
        r.Post("/", createTodo)
        r.Put("/{id}", updateTodo)
        r.Delete("/{id}", deleteTodo)
    })
    return rg
}

func checkErr(err error) {
    if err != nil {
        log.Fatal(err)
    }
}
