package query

import (
	"bytes"
	"database/sql"
	"fmt"

	"github.com/rwcarlsen/cyan/nuc"
)

func Breakout(src, dst string, simid string) error {
	f1, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f1.Close()

	f2, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f2.Close()

	err, _ = io.Copy(dst, src)
	if err != nil {
		return err
	}

	sqls := []string{
		"DELETE FROM Resources WHERE SimID = ?;",
		"DELETE FROM Compositions WHERE SimID = ?;",
		"DELETE FROM Transactions WHERE SimID = ?;",
		"DELETE FROM TransactedResources WHERE SimID = ?;",
		"DELETE FROM Inventories WHERE SimID = ?;",
		"DELETE FROM ResCreators WHERE SimID = ?;",
		"DELETE FROM Agents WHERE SimID = ?;",
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}

}

// Index builds an sql statement for creating a new index on the specified
// table over cols.  The index is named according to the table and cols.
func Index(table string, cols ...string) string {
	var buf bytes.Buffer
	buf.WriteString("CREATE INDEX IF NOT EXISTS ")
	buf.WriteString(table + "_" + cols[0])
	for _, c := range cols[1:] {
		buf.WriteString("_" + c)
	}
	buf.WriteString(" ON " + table + " (" + cols[0] + " ASC")
	for _, c := range cols[1:] {
		buf.WriteString("," + c + " ASC")
	}
	buf.WriteString(");")
	return buf.String()
}

// SimIds returns a list of all simulation ids in the cyclus database for
// conn.
func SimIds(db *sql.DB) (ids []string, err error) {
	sql := "SELECT SimID FROM SimulationTimeInfo"
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		ids = append(ids, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

type SimInfo struct {
	StartTime   int
	Duration    int
	DecayPeriod int
}

func SimStat(db *sql.DB, simid string) (si SimInfo, err error) {
	sql := "SELECT SimulationStart,Duration FROM SimulationTimeInfo WHERE SimID = ?"
	rows, err := db.Query(sql, simid)
	if err != nil {
		return si, err
	}

	for rows.Next() {
		if err := rows.Scan(&si.StartTime, &si.Duration); err != nil {
			return si, err
		}
	}
	if err := rows.Err(); err != nil {
		return si, err
	}
	return si, nil
}

func AllMatCreated(db *sql.DB, simid string) (m nuc.Material, err error) {
	if err := index(db); err != nil {
		return nil, err
	}
	sql := `SELECT cmp.IsoID,SUM(cmp.Quantity * res.Quantity)
	        	FROM Resources As res
				INNER JOIN Compositions AS cmp ON res.StateID = cmp.ID
				INNER JOIN ResCreators AS cre ON res.ID = cre.ResID
				WHERE
					cre.SimID = ? AND cre.SimID = res.SimID AND cre.SimID = cmp.SimID
				GROUP BY cmp.IsoID;`
	return makeMaterial(db, sql, simid)
}

func index(db *sql.DB) error {
	sqls := []string{
		Index("Compositions", "ID", "IsoID", "Quantity"),
	}

	for _, sql := range sqls {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}
	fmt.Println("done indexing")
	return nil
}

// AllMatAt returns the simulation-global material inventory for the specified
// sim id at time t. Use t=-1 to specify end-of-simulation.
func AllMatAt(db *sql.DB, simid string, t int) (m nuc.Material, err error) {
	if err := index(db); err != nil {
		return nil, err
	}

	if t == -1 {
		si, err := SimStat(db, simid)
		if err != nil {
			return nil, err
		}
		t = si.StartTime + si.Duration
	}
	sql := `SELECT cmp.IsoID,SUM(cmp.Quantity * res.Quantity)
				FROM Inventories As inv
				INNER JOIN Resources AS res
				INNER JOIN Compositions AS cmp
				WHERE
					inv.ResID = res.ID AND res.StateID = cmp.ID
					AND inv.StartTime <= ? AND inv.EndTime > ?
					AND res.SimID = inv.SimID AND cmp.SimID = res.SimID AND inv.SimID = ?
				GROUP BY cmp.IsoID;`
	return makeMaterial(db, sql, t, t, simid)
}

func makeMaterial(db *sql.DB, sql string, args ...interface{}) (m nuc.Material, err error) {
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}

	m = nuc.Material{}
	var iso int
	var qty float64
	for rows.Next() {
		if err := rows.Scan(&iso, &qty); err != nil {
			return nil, err
		}
		m[nuc.Nuc(iso)] = nuc.Mass(qty)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
