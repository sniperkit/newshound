package fetch

import (
	"context"
	"time"

	"github.com/NYTimes/sqliface"
	"github.com/jprobinson/newshound"
	pql "github.com/lib/pq"
	"github.com/pkg/errors"
)

type DB interface {
	PutAlert(context.Context, *newshound.NewsAlert) error
	FindAlertsByTimeframe(context.Context, time.Time, time.Time) ([]*newshound.NewsAlert, error)
	FindPossibleLikeAlerts(context.Context, *newshound.NewsAlert) ([]*newshound.NewsAlert, error)
	FindEventsByAlertIDs(context.Context, []int64) ([]*newshound.NewsEvent, error)
	GetAlertsByID(context.Context, []int64) ([]*newshound.NewsAlert, error)
	UpsertEvent(context.Context, *newshound.NewsEvent) error
	DeleteEvents(context.Context, []int64) error
	GetAllAlerts(context.Context) (<-chan *newshound.NewsAlert, error)
}

type pq struct {
	db sqliface.Execer
}

func NewDB(db sqliface.Execer) DB {
	return &pq{db}
}

func (p *pq) PutAlert(ctx context.Context, a *newshound.NewsAlert) error {
	// start transaction ?

	// save alert, get ID from result
	const ins = `INSERT INTO newshound.alert
	(sender_id, url, "timestamp", top_phrases, subject, raw_body, body)
	VALUES 
		((SELECT ID FROM newshound.sender WHERE name = lower($1)), $2, 
		NOW() at time zone 'utc', $3, $4, $5, $6)
	RETURNING id`
	var aid int64
	// pq forces us to query on insert to get the new ID
	err := p.db.QueryRow(ins,
		a.Sender,
		a.ArticleUrl,
		pql.StringArray(a.TopPhrases),
		a.Subject,
		a.RawBody,
		a.Body,
	).Scan(&aid)
	if err != nil {
		return errors.Wrap(err, "unable to insert alert")
	}

	// save sentences
	var tid int64
	for _, s := range a.Sentences {
		sid, err := p.putSentence(ctx, &s, aid)
		if err != nil {
			return errors.Wrap(err, "unable to insert sentence")
		}
		if s.Value == a.TopSentence {
			tid = sid
		}
	}

	// update alert with top sentence ID
	const ups = `UPDATE newshound.alert SET top_sentence = $1 WHERE id = $2`
	_, err = p.db.Exec(ups, tid, aid)
	if err != nil {
		return errors.Wrap(err, "unable to set top sentence ID")
	}

	// end transaction ?
	return nil
}

func (p *pq) putSentence(ctx context.Context, s *newshound.Sentence, aid int64) (int64, error) {
	const ins = `INSERT INTO newshound.sentence
				(text, phrases, alert_id)
				VALUES ($1, $2, $3)
				RETURNING id`
	var id int64
	err := p.db.QueryRowContext(ctx, ins, s.Value, pql.StringArray(s.Phrases), aid).
		Scan(&id)
	if err != nil {
		return 0, errors.Wrap(err, "unable to insert sentence")
	}

	return id, err
}

func (p *pq) FindAlertsByTimeframe(ctx context.Context, start time.Time, end time.Time) ([]*newshound.NewsAlert, error) {
	const qry = `SELECT
				id, sender_id, url, "timestamp", top_phrases, subject, raw_body, body
				FROM
				newshound.alert
				WHERE
				"timestamp" BETWEEN to_timestamp($1) AND to_timestamp($2)`
	rows, err := p.db.QueryContext(ctx, qry, start.Unix(), end.Unix())
	return scanAlerts(rows, err)
}

func scanAlerts(rows sqliface.Rows, err error) ([]*newshound.NewsAlert, error) {
	if err != nil {
		return nil, errors.Wrap(err, "unable to run query")
	}
	var alerts []*newshound.NewsAlert
	for rows.Next() {
		var a newshound.NewsAlert
		var phrases pql.StringArray
		err = rows.Scan(
			&a.ID,
			&a.Sender,
			&a.ArticleUrl,
			&a.Timestamp,
			&phrases,
			&a.Subject,
			&a.RawBody,
			&a.Body,
		)
		if err != nil {
			return nil, errors.Wrap(err, "unable to scan alert")
		}
		a.TopPhrases = phrases

		alerts = append(alerts, &a)
	}
	return alerts, nil
}

func (p *pq) getSentencesByAlertID(ctx context.Context, alertID int64) ([]*newshound.Sentence, error) {
	return nil, nil
}

func (p *pq) FindPossibleLikeAlerts(ctx context.Context, a *newshound.NewsAlert) ([]*newshound.NewsAlert, error) {
	const qry = `SELECT
					id, sender_id, url, "timestamp", top_phrases, subject, raw_body, body
				FROM newshound.alert
				WHERE a.timestamp between $1 and $2
				AND a.id != $3
				AND a.top_phrases @> $4`
	start := a.Timestamp.Add(-eventTimeframe)
	end := a.Timestamp.Add(eventTimeframe)
	rows, err := p.db.QueryContext(ctx, qry, start, end, a.ID, a.Tags)
	return scanAlerts(rows, err)
}

func (p *pq) FindEventsByAlertIDs(context.Context, []int64) ([]*newshound.NewsEvent, error) {
	//	var existingEvents []newshound.NewsEvent
	//	query := bson.M{"news_alerts.alert_id": bson.M{"$in": cluster}}
	//	err = ne.Find(query).All(&existingEvents)
	return nil, nil
}

func (p *pq) GetAlertsByID(context.Context, []int64) ([]*newshound.NewsAlert, error) {
	return nil, nil
}

func (p *pq) UpsertEvent(context.Context, *newshound.NewsEvent) error {
	return nil
}

func (p *pq) DeleteEvents(context.Context, []int64) error {
	return nil
}

func (p *pq) GetAllAlerts(context.Context) (<-chan *newshound.NewsAlert, error) {
	return nil, nil
}
