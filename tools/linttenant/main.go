// Command linttenant menegakkan aturan multi-tenant (make lint-tenant):
//
//  1. Setiap query sqlc (src/modules/*/db/queries/*.sql) yang menyentuh tabel
//     ber-kolom wedding_id wajib memfilter wedding_id di WHERE
//     (INSERT: wajib mengisi kolom wedding_id).
//  2. Setiap tabel ber-kolom wedding_id wajib punya index yang diawali wedding_id.
//
// Pengecualian yang disengaja (mis. lookup publik by kode undangan) diberi komentar
// `-- tenant:ignore <alasan>` di dalam blok query tersebut.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	migrations := flag.String("migrations", "migrations", "folder migration SQL")
	queries := flag.String("queries", "src/modules/*/db/queries/*.sql", "glob file query sqlc")
	flag.Parse()

	problems, err := lint(os.DirFS("."), *migrations, *queries)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lint-tenant:", err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		fmt.Printf("lint-tenant: %d masalah\n", len(problems))
		os.Exit(1)
	}
	fmt.Println("lint-tenant: ok")
}

var (
	reLineComment = regexp.MustCompile(`--[^\n]*`)
	reCreateTable = regexp.MustCompile(`(?i)\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?([a-z_][a-z0-9_.]*)\s*\(`)
	reAlterAdd    = regexp.MustCompile(`(?i)\balter\s+table\s+(?:if\s+exists\s+)?(?:only\s+)?([a-z_][a-z0-9_.]*)\s+add\s+(?:column\s+)?(?:if\s+not\s+exists\s+)?wedding_id\b`)
	reIndex       = regexp.MustCompile(`(?i)\bcreate\s+(?:unique\s+)?index\s+(?:concurrently\s+)?(?:if\s+not\s+exists\s+)?(?:[a-z_][a-z0-9_]*\s+)?on\s+(?:only\s+)?([a-z_][a-z0-9_.]*)\s*(?:using\s+[a-z]+\s*)?\(\s*wedding_id\b`)
	reKeyInBody   = regexp.MustCompile(`(?i)\b(?:primary\s+key|unique)\s*\(\s*wedding_id\b`)
	reColumnKey   = regexp.MustCompile(`(?im)^\s*wedding_id\b[^,]*\b(?:primary\s+key|unique)\b`)
	reWeddingID   = regexp.MustCompile(`(?i)\bwedding_id\b`)
	reTableRef    = regexp.MustCompile(`(?i)\b(?:from|join|update|into)\s+(?:only\s+)?([a-z_][a-z0-9_.]*)`)
	reWhere       = regexp.MustCompile(`(?i)\bwhere\b`)
	reQueryName   = regexp.MustCompile(`(?m)^--\s*name:\s*(\S+)`)
)

type tableInfo struct {
	file    string
	indexed bool
}

func lint(fsys fs.FS, migrationsDir, queryGlob string) ([]string, error) {
	tenants, err := tenantTables(fsys, migrationsDir)
	if err != nil {
		return nil, err
	}

	var problems []string
	names := make([]string, 0, len(tenants))
	for name := range tenants {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !tenants[name].indexed {
			problems = append(problems, fmt.Sprintf("%s: tabel %q punya wedding_id tapi tidak ada index yang diawali wedding_id", tenants[name].file, name))
		}
	}

	files, err := fs.Glob(fsys, queryGlob)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		problems = append(problems, checkQueries(f, string(b), tenants)...)
	}
	return problems, nil
}

func tenantTables(fsys fs.FS, dir string) (map[string]*tableInfo, error) {
	files, err := fs.Glob(fsys, filepath.ToSlash(filepath.Join(dir, "*.sql")))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	tables := map[string]*tableInfo{}
	var indexed []string
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		sql := upSection(string(b))

		for _, m := range reCreateTable.FindAllStringSubmatchIndex(sql, -1) {
			name := normalize(sql[m[2]:m[3]])
			body := parenBody(sql, m[1]-1)
			if !reWeddingID.MatchString(body) {
				continue
			}
			tables[name] = &tableInfo{file: f}
			if reKeyInBody.MatchString(body) || reColumnKey.MatchString(body) {
				indexed = append(indexed, name)
			}
		}
		for _, m := range reAlterAdd.FindAllStringSubmatch(sql, -1) {
			name := normalize(m[1])
			if tables[name] == nil {
				tables[name] = &tableInfo{file: f}
			}
		}
		for _, m := range reIndex.FindAllStringSubmatch(sql, -1) {
			indexed = append(indexed, normalize(m[1]))
		}
	}
	for _, name := range indexed {
		if t := tables[name]; t != nil {
			t.indexed = true
		}
	}
	return tables, nil
}

func checkQueries(file, content string, tenants map[string]*tableInfo) []string {
	var problems []string
	locs := reQueryName.FindAllStringSubmatchIndex(content, -1)
	for i, loc := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := content[loc[0]:end]
		name := content[loc[2]:loc[3]]
		line := strings.Count(content[:loc[0]], "\n") + 1

		if strings.Contains(block, "tenant:ignore") {
			continue
		}
		sql := reLineComment.ReplaceAllString(block, "")

		var touched []string
		for _, m := range reTableRef.FindAllStringSubmatch(sql, -1) {
			if t := normalize(m[1]); tenants[t] != nil {
				touched = append(touched, t)
			}
		}
		if len(touched) == 0 {
			continue
		}

		isInsert := strings.HasPrefix(strings.ToLower(strings.TrimSpace(sql)), "insert")
		ok := false
		if isInsert {
			ok = reWeddingID.MatchString(sql)
		} else if w := reWhere.FindStringIndex(sql); w != nil {
			ok = reWeddingID.MatchString(sql[w[0]:])
		}
		if !ok {
			what := "tidak memfilter wedding_id di WHERE"
			if isInsert {
				what = "tidak mengisi kolom wedding_id"
			}
			problems = append(problems, fmt.Sprintf("%s:%d: query %s menyentuh tabel tenant %v tapi %s (tambahkan filter, atau `-- tenant:ignore <alasan>`)",
				file, line, name, touched, what))
		}
	}
	return problems
}

// upSection mengambil bagian `-- +goose Up` saja, tanpa komentar.
func upSection(sql string) string {
	if i := strings.Index(sql, "-- +goose Down"); i >= 0 {
		sql = sql[:i]
	}
	return reLineComment.ReplaceAllString(sql, "")
}

// parenBody mengembalikan isi kurung yang dibuka di posisi open.
func parenBody(s string, open int) string {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[open+1 : i]
			}
		}
	}
	return s[open+1:]
}

func normalize(name string) string {
	name = strings.ToLower(name)
	return strings.TrimPrefix(name, "public.")
}
