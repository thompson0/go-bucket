package db

import (
	"database/sql"
	"github.com/mattn/go-sqlite3"
)


func Init() (*sql_DB, error){
	db,err := sql.Open("sqlite3", "../buckets.db")
	if err != nil {
		return nil, err
	}


	query := `
	CREATE TABLE IF NOT EXISTS buckets (
		url TEXT PRIMARY KEY,
		exist BOOLEAN,
		public BOOLEAN,
		status INTEGER,
		region TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_,err = db.Exec(query)
	return db, err
}

func Save(db *sql.DB, url string ,result BucketTest){
	query := `
	INSERT OR REPLACE INTO buckets (url, exist, public, status, region)
	VALUES (?, ?, ?, ?, ?)
	`

	db.Exec(query,url,result.Exist, result.Public, result.StatusCode, result.Region)
}

func Get(db *sql_DB,url string) (BucketTest,bool){
	query := `
	SELECT exist, public, status, region
	FROM buckets
	WHERE url = ?
	`

	row := db.QueryRow(query,url)

	var res BucketTest
	err := row.Scan(&res.Exist, &res.Public, &res.StatusCode, &res.Region)

	if err != nil{
		return BucketTest{}, false
	}
	return res, true
}