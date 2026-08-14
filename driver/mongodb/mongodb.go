package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type Driver struct {
	client *mongo.Client
	db     *mongo.Database
	mu     sync.Mutex
}

type mongoTx struct {
	d *Driver
}

func (t *mongoTx) Exec(ctx context.Context, cmd string) error {
	return t.d.Exec(ctx, cmd)
}

func New(ctx context.Context, uri, database string) (*Driver, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongodb: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}
	return &Driver{client: client, db: client.Database(database)}, nil
}

func (d *Driver) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	return migrate.NewSchemaModel("nosql"), nil
}

func (d *Driver) Exec(ctx context.Context, cmd string) error {
	doc, err := parseJSONToBSOND(cmd)
	if err != nil {
		return fmt.Errorf("mongodb: parse: %w", err)
	}
	if len(doc) == 0 {
		return nil
	}
	cmdName := doc[0].Key
	res := d.db.RunCommand(ctx, doc)
	if err := res.Err(); err != nil {
		return fmt.Errorf("mongodb: %s: %w", cmdName, err)
	}
	return nil
}

// parseJSONToBSOND converts a JSON object string to an ordered bson.D,
// preserving key order so MongoDB commands are correctly formed.
func parseJSONToBSOND(cmd string) (bson.D, error) {
	dec := json.NewDecoder(strings.NewReader(cmd))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if t != json.Delim('{') {
		return nil, fmt.Errorf("expected object, got %T", t)
	}
	var doc bson.D
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := kt.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", kt)
		}
		var val any
		if err := dec.Decode(&val); err != nil {
			return nil, err
		}
		doc = append(doc, bson.E{Key: key, Value: val})
	}
	// consume closing }
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *Driver) ExecTx(ctx context.Context, fn func(tx migrate.Tx) error) error {
	return fn(&mongoTx{d: d})
}

func (d *Driver) AcquireLock(ctx context.Context) error {
	col := d.db.Collection("_migrate_lock")
	doc := bson.M{"_id": "lock", "locked_at": time.Now().UTC()}

	_, err := col.InsertOne(ctx, doc)
	if err == nil {
		return nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("mongodb: acquire lock: %w", err)
	}

	var existing bson.M
	if err := col.FindOne(ctx, bson.M{"_id": "lock"}).Decode(&existing); err != nil {
		return fmt.Errorf("mongodb: read lock: %w", err)
	}
	if ts, ok := existing["locked_at"].(time.Time); ok && time.Since(ts) < 5*time.Minute {
		return fmt.Errorf("mongodb: lock already held")
	}

	_, err = col.UpdateOne(ctx, bson.M{"_id": "lock"}, bson.M{"$set": bson.M{"locked_at": time.Now().UTC()}})
	return err
}

func (d *Driver) ReleaseLock(ctx context.Context) error {
	_, err := d.db.Collection("_migrate_lock").DeleteOne(ctx, bson.M{"_id": "lock"})
	return err
}

func (d *Driver) EnsureHistoryTable(ctx context.Context) error {
	col := d.db.Collection("_migrate_history")
	model := mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_version"),
	}
	_, err := col.Indexes().CreateOne(ctx, model)
	if err != nil {
		return fmt.Errorf("mongodb: ensure history index: %w", err)
	}
	return nil
}

func (d *Driver) LoadHistory(ctx context.Context) ([]migrate.MigrationRecord, error) {
	cur, err := d.db.Collection("_migrate_history").Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "version", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []migrate.MigrationRecord
	for cur.Next(ctx) {
		var r struct {
			Version      int64  `bson:"version"`
			Name         string `bson:"name"`
			Direction    string `bson:"direction"`
			Checksum     string `bson:"checksum"`
			AppliedAt    string `bson:"applied_at"`
			ExecutionMS  int64  `bson:"execution_ms"`
			Status       string `bson:"status"`
			ErrorMessage string `bson:"error_message"`
		}
		if err := cur.Decode(&r); err != nil {
			return nil, err
		}
		out = append(out, migrate.MigrationRecord{
			Version:      r.Version,
			Name:         r.Name,
			Direction:    r.Direction,
			Checksum:     r.Checksum,
			AppliedAt:    r.AppliedAt,
			ExecutionMS:  r.ExecutionMS,
			Status:       r.Status,
			ErrorMessage: r.ErrorMessage,
		})
	}
	return out, cur.Err()
}

func (d *Driver) RecordMigration(ctx context.Context, r migrate.MigrationRecord) error {
	_, err := d.db.Collection("_migrate_history").InsertOne(ctx, bson.M{
		"version":       r.Version,
		"name":          r.Name,
		"direction":     r.Direction,
		"checksum":      r.Checksum,
		"applied_at":    r.AppliedAt,
		"execution_ms":  r.ExecutionMS,
		"status":        r.Status,
		"error_message": r.ErrorMessage,
	})
	return err
}

func (d *Driver) DriverName() string { return "mongodb" }

func (d *Driver) DatabaseVersion(ctx context.Context) (string, error) {
	var buildInfo bson.M
	err := d.db.RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&buildInfo)
	if err != nil {
		return "", err
	}
	if v, ok := buildInfo["version"].(string); ok {
		return v, nil
	}
	return "unknown", nil
}

func (d *Driver) CheckNulls(ctx context.Context, table, column string) (int64, error) {
	col := d.db.Collection(table)
	count, err := col.CountDocuments(ctx, bson.M{column: nil})
	return count, err
}

func (d *Driver) Close() error {
	return d.client.Disconnect(context.Background())
}

var _ migrate.Driver = (*Driver)(nil)
