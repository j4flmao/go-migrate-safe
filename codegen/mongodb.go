package codegen

import (
	"fmt"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

func RenderMongoDB(op *migrate.Operation) (string, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return renderMongoCreateCollection(op), nil
	case migrate.OpDropTable:
		return fmt.Sprintf(`{"drop": %q}`, op.Table), nil
	case migrate.OpRenameTable:
		return fmt.Sprintf(`{"renameCollection": %q, "to": %q}`, op.Table, op.NewTable.Name), nil
	case migrate.OpAddIndex:
		return renderMongoAddIndex(op), nil
	case migrate.OpDropIndex:
		return fmt.Sprintf(`{"dropIndexes": %q, "index": %q}`, op.Table, op.Index), nil
	case migrate.OpAddColumn, migrate.OpDropColumn, migrate.OpAlterColumn,
		migrate.OpRenameColumn, migrate.OpAddConstraint, migrate.OpDropConstraint:
		return "", nil
	}
	return "", fmt.Errorf("unsupported op kind for mongodb: %s", op.Kind)
}

func renderMongoCreateCollection(op *migrate.Operation) string {
	return fmt.Sprintf(`{"create": %q}`, op.Table)
}

func renderMongoAddIndex(op *migrate.Operation) string {
	if op.IndexDef == nil {
		return ""
	}
	keys := ""
	for i, c := range op.IndexDef.Columns {
		if i > 0 {
			keys += ", "
		}
		keys += fmt.Sprintf("%q: 1", c)
	}
	unique := ""
	if op.IndexDef.Unique {
		unique = `, "unique": true`
	}
	return fmt.Sprintf(`{"createIndexes": %q, "indexes": [{"key": {%s}, "name": %q%s}]}`,
		op.Table, keys, op.IndexDef.Name, unique)
}
