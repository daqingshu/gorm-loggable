package loggable

import (
	"encoding/json"
	"reflect"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

var im = newIdentityManager()

const (
	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

// UpdateDiff UpdateDiff
type UpdateDiff map[string]interface{}

// Hook for after_query.
func (p *Plugin) trackEntity(db *gorm.DB) {
	if !isLoggable(db.Statement.Dest) || !isEnabled(db.Statement.Dest) {
		return
	}

	v := reflect.Indirect(reflect.ValueOf(db.Statement.Dest))

	pf := db.Statement.Schema.PrioritizedPrimaryField
	fieldValue, _ := pf.ValueOf(db.Statement.ReflectValue)
	//pkName := db.PrimaryField().Name
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			sv := reflect.Indirect(v.Index(i))
			el := sv.Interface()
			if !isLoggable(el) {
				continue
			}

			im.save(el, sv.FieldByName(pf.Name))
		}
		return
	}

	m := v.Interface()
	if !isLoggable(m) {
		return
	}

	im.save(db.Statement.Dest, fieldValue)
}

// Hook for after_create.
func (p *Plugin) addCreated(db *gorm.DB) {
	if isLoggable(db.Statement.Dest) && isEnabled(db.Statement.Dest) {
		_ = addRecord(db, actionCreate)
	}
}

// Hook for after_update.
func (p *Plugin) addUpdated(db *gorm.DB) {
	if !isLoggable(db.Statement.Dest) || !isEnabled(db.Statement.Dest) {
		return
	}
	pf := db.Statement.Schema.PrioritizedPrimaryField
	fieldValue, _ := pf.ValueOf(db.Statement.ReflectValue)
	if p.opts.lazyUpdate {
		record, err := p.GetLastRecord(interfaceToString(fieldValue), false)
		if err == nil {
			if isEqual(record.RawObject, db.Statement.Dest, p.opts.lazyUpdateFields...) {
				return
			}
		}
	}

	_ = addUpdateRecord(db, p.opts)
}

// Hook for after_delete.
func (p *Plugin) addDeleted(db *gorm.DB) {
	if isLoggable(db.Statement.Dest) && isEnabled(db.Statement.Dest) {
		_ = addRecord(db, actionDelete)
	}
}

func addUpdateRecord(db *gorm.DB, opts options) error {
	cl, err := newChangeLog(db, actionUpdate)
	if err != nil {
		return err
	}

	if opts.computeDiff {
		diff := computeUpdateDiff(db)

		if diff != nil {
			jd, err := json.Marshal(diff)
			if err != nil {
				return err
			}

			cl.RawDiff = string(jd)
		}
	}

	return db.Session(&gorm.Session{NewDB: true}).Create(cl).Error
}

func newChangeLog(db *gorm.DB, action string) (*ChangeLog, error) {
	rawObject, err := json.Marshal(db.Statement.Dest)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	pf := db.Statement.Schema.PrioritizedPrimaryField
	fieldValue, _ := pf.ValueOf(db.Statement.ReflectValue)
	return &ChangeLog{
		ID:         id,
		Action:     action,
		ObjectID:   interfaceToString(fieldValue),
		ObjectType: db.Statement.Schema.ModelType.Name(),
		RawObject:  string(rawObject),
		RawMeta:    string(fetchChangeLogMeta(db)),
		RawDiff:    "null",
	}, nil
}

// Writes new change log row to db.
func addRecord(db *gorm.DB, action string) error {
	cl, err := newChangeLog(db, action)
	if err != nil {
		return nil
	}

	return db.Session(&gorm.Session{NewDB: true}).Create(cl).Error
}

func computeUpdateDiff(db *gorm.DB) UpdateDiff {
	pf := db.Statement.Schema.PrioritizedPrimaryField
	fieldValue, _ := pf.ValueOf(db.Statement.ReflectValue)
	old := im.get(db.Statement.Dest, fieldValue)
	if old == nil {
		return nil
	}

	ov := reflect.ValueOf(old)
	nv := reflect.Indirect(reflect.ValueOf(db.Statement.Dest))
	names := getLoggableFieldNames(old)

	diff := make(UpdateDiff)

	for _, name := range names {
		ofv := ov.FieldByName(name).Interface()
		nfv := nv.FieldByName(name).Interface()
		if ofv != nfv {
			diff[name] = nfv
		}
	}

	return diff
}
