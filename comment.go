package main

import (
	"database/sql"
	"time"
)

type Comment struct {
	CommentId       int64
	ItemTypeId      int64
	ItemId          int64
	ProfileId       int64
	Created         time.Time
	InReplyTo       int64
	IsVisible       bool
	IsModerated     bool
	IsDeleted       bool
	AttachmentCount int64
}

type CommentRevision struct {
	RevisionId int64
	CommentId  int64
	ProfileId  int64
	Raw        string
	HTML       string
	Created    time.Time
	IsCurrent  bool
}

/*
func ImportComments() {
	// Load comments.
	eCommMap, err := walk.WalkExports(config.Rootpath, "comments")
	if err != nil {
		exitWithError(err, errors)
	}

	var commentKeys []int
	for key := range eCommMap {
		commentKeys = append(commentKeys, key)
	}
	sort.Ints(commentKeys)

	for _, CommID := range commentKeys {
		bytes, err := ioutil.ReadFile(eCommMap[CommID])
		if err != nil {
			errors = append(errors, err)
			log.Print(err)
			continue
		}
		eComm := exports.Comment{}
		err = json.Unmarshal(bytes, &eComm)
		if err != nil {
			errors = append(errors, err)
			log.Print(err)
			continue
		}

		// Look up the author profile based on the old user ID.
		_, ok := pMap[eComm.Author]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"Exported user ID %d does not have an imported profile, skipped comment %d\n",
				eComm.Author,
				CommID,
			))
			log.Print(err)
			continue
		}

		// Look up the imported conversation ID based on the old ID. Assumes comments are only on conversations.
		_, ok = cMap[int(eComm.Association.OnID)]
		if !ok {
			errors = append(errors, fmt.Errorf(
				"Exported thread ID %d does not have an imported conversation, skipped comment %d\n",
				eComm.Association.OnID,
				CommID,
			))
			log.Print(err)
			continue
		}

		// InReplyTo
		// Store comment.

}
*/
func StoreComment(db *sql.DB, c Comment) (int64, error) {

	tx, err := db.Begin()
	defer tx.Rollback()
	if err != nil {
		return 0, err
	}

	var cID int64
	err = tx.QueryRow(
		`INSERT INTO comments () VALUES () RETURNING conversation_id;`,
	).Scan(&cID)

	if err != nil {
		return cID, err
	}
	err = tx.Commit()
	return cID, err

}

func StoreRevision() {}
