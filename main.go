package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
)

type Student struct {
	NIM     string `json:"nim"`
	Name    string `json:"name"`
	Age     uint16 `json:"age"`
	Address string `json:"address"`
}

var errDataNotFound = errors.New("data not found")
var errInternalServer = errors.New("internal server error")

type Datastore struct {
	// StudentMap map[string]Student
	StudentSQLite *sql.DB
}

func (ds *Datastore) Save(student Student) error {
	stmt, err := ds.StudentSQLite.Prepare("INSERT INTO students(nim, name, age, address) values(?,?,?,?)")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(student.NIM, student.Name, student.Age, student.Address)
	if err != nil {
		return err
	}

	return nil
}

func (ds *Datastore) DeleteByNIM(nim string) error {
	sqlStatement := `DELETE FROM students WHERE nim = $1;`
	_, err := ds.StudentSQLite.Exec(sqlStatement, nim)
	return err
}

func (ds *Datastore) UpdateByNIM(student Student) error {

	stmt, _ := ds.StudentSQLite.Prepare("UPDATE students SET name = ?, age = ?, address = ? WHERE nim = ?")
	defer stmt.Close()

	res, err := stmt.Exec(student.Name, student.Age, student.Address, student.NIM)
	log.Println(res.RowsAffected())
	return err
}

func (ds *Datastore) FindAll() []Student {
	var students []Student
	rows, _ := ds.StudentSQLite.Query("SELECT * FROM students")
	defer rows.Close()

	for rows.Next() {
		var student Student
		rows.Scan(&student.NIM, &student.Name, &student.Age, &student.Address)
		students = append(students, student)
	}

	return students
}

func (ds *Datastore) FindByNIM(nim string) (Student, error) {
	var student Student
	sqlStatement := `SELECT nim, name,age,address FROM students WHERE nim=$1;`
	row := ds.StudentSQLite.QueryRow(sqlStatement, nim)
	err := row.Scan(&student.NIM, &student.Name, &student.Age, &student.Address)
	if err != nil {
		if err == sql.ErrNoRows {
			return Student{}, errDataNotFound
		}
		return Student{}, errInternalServer
	}

	return student, nil
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	db, err := sql.Open("sqlite3", "./students.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlStmt := `create table if not exists students (nim text not null primary key, name text not null, age INTEGER not null, address TEXT not null);`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return
	}

	datastore := Datastore{
		StudentSQLite: db,
	}

	r.Post("/students", func(w http.ResponseWriter, r *http.Request) {
		var student Student
		err := json.NewDecoder(r.Body).Decode(&student)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = datastore.Save(student)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(student.NIM))
	})

	r.Delete("/students/{nim}", func(w http.ResponseWriter, r *http.Request) {
		nim := chi.URLParam(r, "nim")
		err := datastore.DeleteByNIM(nim)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	r.Put("/students", func(w http.ResponseWriter, r *http.Request) {
		var student Student
		err := json.NewDecoder(r.Body).Decode(&student)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = datastore.UpdateByNIM(student)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/students", func(w http.ResponseWriter, r *http.Request) {
		students := datastore.FindAll()
		w.WriteHeader(http.StatusOK)
		studentJSON, _ := json.Marshal(students)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(studentJSON))
	})

	r.Get("/students/{nim}", func(w http.ResponseWriter, r *http.Request) {
		nim := chi.URLParam(r, "nim")
		student, err := datastore.FindByNIM(nim)

		if err != nil {
			if errors.Is(err, errDataNotFound) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(err.Error()))
				return
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		}

		studentJSON, _ := json.Marshal(student)
		w.WriteHeader(http.StatusOK)
		w.Write(studentJSON)
	})

	log.Println("server start on port :3030")
	http.ListenAndServe(":3030", r)
}
