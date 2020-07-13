package gitqlite

import (
	"database/sql"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	fixtureRepoCloneURL = "https://github.com/augmentable-dev/tickgit"
	fixtureRepo         *git.Repository
	instance            *GitQLite
)

func TestMain(m *testing.M) {
	close, err := initFixtureRepo()
	if err != nil {
		panic(err)
	}
	code := m.Run()
	close()
	os.Exit(code)
}

func initFixtureRepo() (func() error, error) {
	dir, err := ioutil.TempDir("", "repo")
	if err != nil {
		return nil, err
	}

	fixtureRepo, err = git.PlainClone(dir, false, &git.CloneOptions{
		URL: fixtureRepoCloneURL,
	})
	if err != nil {
		return nil, err
	}

	instance, err = New(dir)
	if err != nil {
		return nil, err
	}

	return func() error {
		err := os.RemoveAll(dir)
		if err != nil {
			return err
		}
		return nil
	}, nil
}

func TestModuleInitialization(t *testing.T) {
	if instance.DB == nil {
		t.Fatal("expected non-nil DB, got nil")
	}
}

func TestCommitCounts(t *testing.T) {
	headRef, err := fixtureRepo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commitChecker, err := fixtureRepo.Log(&git.LogOptions{
		From:  headRef.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		t.Fatal(err)
	}

	commitCount := 0
	err = commitChecker.ForEach(func(c *object.Commit) error {
		commitCount++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	//checks commits
	rows, err := instance.DB.Query("SELECT * FROM commits")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	expected := 14
	if len(columns) != expected {
		t.Fatalf("expected %d columns, got: %d", expected, len(columns))
	}
	numRows := getRowsCount(rows)

	expected = commitCount
	if numRows != expected {
		t.Fatalf("expected %d rows got: %d", expected, numRows)
	}
	rows, err = instance.DB.Query("SELECT id, author_email FROM commits")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	commitChecker, err = fixtureRepo.Log(&git.LogOptions{
		From:  headRef.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	count, match, err := checkRowContents(rows, commitChecker)
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatalf("Contents does not match at row number: %d ", count)
	}
}

func TestFileCounts(t *testing.T) {
	headRef, err := fixtureRepo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commitChecker, err := fixtureRepo.Log(&git.LogOptions{
		From:  headRef.Hash(),
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	numFiles := 0
	err = commitChecker.ForEach(func(c *object.Commit) error {
		numFiles++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	fileRows, err := instance.DB.Query("SELECT DISTINCT commit_id FROM files")
	if err != nil {
		t.Fatal(err)
	}
	defer fileRows.Close()
	numFileRows := getRowsCount(fileRows)
	if numFileRows != numFiles {
		t.Fatalf("expected %d rows got : %d", numFiles, numFileRows)
	}
	columnQuery, err := instance.DB.Query("SELECT * FROM files LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	columns, err := columnQuery.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 6 {
		t.Fatalf("expected %d columns got : %d", 6, len(columns))
	}
}

func TestRefCounts(t *testing.T) {
	refChecker, err := fixtureRepo.References()
	if err != nil {
		t.Fatal(err)
	}
	refCount := 0
	err = refChecker.ForEach(func(r *plumbing.Reference) error {
		refCount++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	//check refs
	refRows, err := instance.DB.Query("SELECT * FROM refs")
	if err != nil {
		t.Fatal(err)
	}

	columns, err := refRows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	expected := 3
	if len(columns) != expected {
		t.Fatalf("expected %d columns, got: %d", expected, len(columns))
	}
	numRows := getRowsCount(refRows)
	if numRows != refCount {
		t.Fatalf("expected %d rows got : %d", refCount, numRows)
	}
}

func getRowsCount(rows *sql.Rows) int {
	count := 0
	for rows.Next() {
		count++
	}

	return count
}
func checkRowContents(rows *sql.Rows, iter object.CommitIter) (int, bool, error) {
	var (
		count int = 0
	)
	columns, err := rows.Columns()
	if err != nil {
		return count, false, err
	}

	pointers := make([]interface{}, len(columns))
	container := make([]sql.NullString, len(columns))

	for i := range pointers {
		pointers[i] = &container[i]
	}

	for rows.Next() {
		current, err := iter.Next()
		if err != nil {
			if err == io.EOF {
			} else {
				return count, false, err
			}
		}
		err = rows.Scan(pointers...)
		if err != nil {
			return count, false, err
		}

		r := make([]string, len(columns))
		for i, c := range container {
			if c.Valid {
				r[i] = c.String
			} else {
				r[i] = "NULL"
			}
		}
		if r[0] != current.Hash.String() || r[1] != current.Author.Email {
			return count, false, err
		}

	}
	return count, true, err

}
