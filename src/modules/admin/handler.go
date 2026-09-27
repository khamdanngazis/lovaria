package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/domain"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc *Service
}

func idParam(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusNotFound)
	}
	return id, nil
}

func pageParam(c echo.Context) int {
	p, _ := strconv.Atoi(c.QueryParam("page"))
	return max(1, p)
}

// flash: pesan setelah aksi (redirect POST → GET).
type flash struct{ Notice, Error string }

func flashOf(c echo.Context) flash {
	return flash{Notice: c.QueryParam("ok"), Error: c.QueryParam("err")}
}

// back mengalihkan ke path dengan pesan sukses / error. Error pengguna
// (validasi, transisi, data tidak ada) ditampilkan; error lain diteruskan.
func back(c echo.Context, path, notice string, err error) error {
	v := url.Values{}
	if err != nil {
		var ve ValidationError
		var te *wedding.TransitionError
		var ce *wedding.ChecklistError
		switch {
		case errors.As(err, &ve):
			for _, m := range ve {
				v.Set("err", m)
			}
		case errors.As(err, &te), errors.As(err, &ce), errors.Is(err, ErrNotFound), errors.Is(err, ErrPackageUsed):
			v.Set("err", err.Error())
		default:
			return err
		}
	} else if notice != "" {
		v.Set("ok", notice)
	}
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	return c.Redirect(http.StatusSeeOther, path)
}

// ---------- Ringkasan ----------

func (h *Handler) Home(c echo.Context) error {
	ctx := c.Request().Context()
	var d homeData
	var err error
	users, err := h.svc.Users.SearchUsers(ctx, "", 1, 1)
	if err != nil {
		return err
	}
	d.Users = users.Total
	if d.ByStatus, err = h.svc.Weddings.CountByStatus(ctx); err != nil {
		return err
	}
	for _, n := range d.ByStatus {
		d.Weddings += n
	}
	if d.Storage, err = h.svc.Weddings.StorageTotal(ctx); err != nil {
		return err
	}
	if d.ActiveDomains, err = h.svc.Domains.ActiveCount(ctx); err != nil {
		return err
	}
	d.QuotaWarn = h.svc.Domains.QuotaWarn()
	if d.Audit, _, err = h.svc.AuditLog(ctx, "", 1); err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, homePage(d))
}

// ---------- Customers ----------

func (h *Handler) Users(c echo.Context) error {
	q := c.QueryParam("q")
	p, err := h.svc.Users.SearchUsers(c.Request().Context(), q, pageParam(c), PerPage)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, usersPage(q, p))
}

func (h *Handler) User(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := idParam(c, "userID")
	if err != nil {
		return err
	}
	u, err := h.svc.Users.GetUser(ctx, id)
	if errors.Is(err, auth.ErrUserNotFound) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return err
	}
	ws, err := h.svc.Weddings.ListWeddingsByOwner(ctx, id)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, userPage(u, ws, flashOf(c)))
}

// POST /admin/users/:userID/disabled (disabled=1|0)
func (h *Handler) SetUserDisabled(c echo.Context) error {
	id, err := idParam(c, "userID")
	if err != nil {
		return err
	}
	disabled := c.FormValue("disabled") == "1"
	_, err = h.svc.SetUserDisabled(c.Request().Context(), id, disabled)
	msg := "Akun diaktifkan kembali."
	if disabled {
		msg = "Akun dinonaktifkan dan dikeluarkan dari semua sesi."
	}
	return back(c, "/admin/users/"+id.String(), msg, err)
}

// ---------- Weddings ----------

func (h *Handler) Weddings(c echo.Context) error {
	ctx := c.Request().Context()
	f := wedding.AdminFilter{
		Status: c.QueryParam("status"), From: c.QueryParam("from"), To: c.QueryParam("to"),
		Q: c.QueryParam("q"), Sort: c.QueryParam("sort"), Page: pageParam(c),
	}
	p, err := h.svc.Weddings.AdminList(ctx, f, PerPage)
	if err != nil {
		return err
	}
	owners := map[uuid.UUID]string{}
	for _, w := range p.Weddings {
		if _, seen := owners[w.OwnerUserID]; seen {
			continue
		}
		if u, err := h.svc.Users.GetUser(ctx, w.OwnerUserID); err == nil {
			owners[w.OwnerUserID] = u.Email
		}
	}
	return web.Render(c, http.StatusOK, weddingsPage(f, p, owners))
}

func (h *Handler) Wedding(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := idParam(c, "weddingID")
	if err != nil {
		return err
	}
	d := weddingData{Flash: flashOf(c)}
	if d.W, err = h.svc.Weddings.GetWedding(ctx, id); errors.Is(err, wedding.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound)
	} else if err != nil {
		return err
	}
	if d.Couple, err = h.svc.Weddings.GetCouple(ctx, id); err != nil {
		return err
	}
	if d.Owner, err = h.svc.Users.GetUser(ctx, d.W.OwnerUserID); err != nil {
		return err
	}
	if d.Guests, err = h.svc.Guests.Stats(ctx, id); err != nil {
		return err
	}
	if d.Usage, err = h.svc.Gallery.StorageUsage(ctx, id); err != nil {
		return err
	}
	switch dm, err := h.svc.Domains.Get(ctx, id); {
	case errors.Is(err, domain.ErrNotFound):
	case err != nil:
		return err
	default:
		d.Domain = &dm
	}
	if d.Package, d.HasPackage, err = h.svc.WeddingPackage(ctx, id); err != nil {
		return err
	}
	if d.Packages, err = h.svc.ListPackages(ctx); err != nil {
		return err
	}
	if d.History, err = h.svc.Weddings.History(ctx, id, 10); err != nil {
		return err
	}
	if d.Audit, _, err = h.svc.AuditLog(ctx, id.String(), 1); err != nil {
		return err
	}
	d.Transitions = wedding.TargetsFor(d.W.Status, wedding.ActorAdmin)
	return web.Render(c, http.StatusOK, weddingPage(d))
}

// POST /admin/weddings/:weddingID/status (status=…)
func (h *Handler) SetWeddingStatus(c echo.Context) error {
	id, err := idParam(c, "weddingID")
	if err != nil {
		return err
	}
	to := c.FormValue("status")
	_, err = h.svc.SetWeddingStatus(c.Request().Context(), id, to)
	return back(c, "/admin/weddings/"+id.String(), "Status diubah menjadi "+wedding.StatusLabel(to)+".", err)
}

// POST /admin/weddings/:weddingID/package (package_id kosong = default)
func (h *Handler) AssignPackage(c echo.Context) error {
	id, err := idParam(c, "weddingID")
	if err != nil {
		return err
	}
	pkg := uuid.Nil
	if v := c.FormValue("package_id"); v != "" {
		if pkg, err = uuid.Parse(v); err != nil {
			return back(c, "/admin/weddings/"+id.String(), "", ErrNotFound)
		}
	}
	err = h.svc.AssignPackage(c.Request().Context(), id, pkg)
	return back(c, "/admin/weddings/"+id.String(), "Paket diperbarui.", err)
}

// POST /admin/weddings/:weddingID/view — mulai mode lihat-saja.
func (h *Handler) StartView(c echo.Context) error {
	id, err := idParam(c, "weddingID")
	if err != nil {
		return err
	}
	if err := h.svc.StartView(c, id); err != nil {
		return back(c, "/admin/weddings", "", err)
	}
	return c.Redirect(http.StatusSeeOther, "/dashboard/weddings/"+id.String())
}

// POST /admin/view/stop
func (h *Handler) StopView(c echo.Context) error {
	id, err := h.svc.StopView(c)
	if err != nil {
		return err
	}
	if id == uuid.Nil {
		return c.Redirect(http.StatusSeeOther, "/admin/weddings")
	}
	return c.Redirect(http.StatusSeeOther, "/admin/weddings/"+id.String())
}

// ---------- Themes ----------

func (h *Handler) Themes(c echo.Context) error {
	ctx := c.Request().Context()
	counts, err := h.svc.Weddings.CountByTheme(ctx)
	if err != nil {
		return err
	}
	off, err := h.svc.Themes.Disabled(ctx)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, themesPage(theme.All(), counts, off, flashOf(c)))
}

// POST /admin/themes/:themeID (enabled=1|0)
func (h *Handler) SetThemeEnabled(c echo.Context) error {
	enabled := c.FormValue("enabled") == "1"
	err := h.svc.SetThemeEnabled(c.Request().Context(), c.Param("themeID"), enabled)
	msg := "Tema dinonaktifkan untuk pasangan baru."
	if enabled {
		msg = "Tema diaktifkan."
	}
	return back(c, "/admin/themes", msg, err)
}

// ---------- Packages ----------

func (h *Handler) Packages(c echo.Context) error {
	ps, err := h.svc.ListPackages(c.Request().Context())
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, packagesPage(ps, flashOf(c)))
}

func packageInput(c echo.Context) PackageInput {
	return PackageInput{Name: c.FormValue("name"), StorageMB: c.FormValue("storage_mb"), ArchiveDays: c.FormValue("archive_days"), PriceDisplay: c.FormValue("price_display")}
}

func (h *Handler) CreatePackage(c echo.Context) error {
	_, err := h.svc.CreatePackage(c.Request().Context(), packageInput(c))
	return back(c, "/admin/packages", "Paket dibuat.", err)
}

func (h *Handler) UpdatePackage(c echo.Context) error {
	id, err := idParam(c, "packageID")
	if err != nil {
		return err
	}
	_, err = h.svc.UpdatePackage(c.Request().Context(), id, packageInput(c))
	return back(c, "/admin/packages", "Paket diperbarui.", err)
}

func (h *Handler) DeletePackage(c echo.Context) error {
	id, err := idParam(c, "packageID")
	if err != nil {
		return err
	}
	return back(c, "/admin/packages", "Paket dihapus.", h.svc.DeletePackage(c.Request().Context(), id))
}

// ---------- Storage, domain, audit ----------

func (h *Handler) Storage(c echo.Context) error {
	ctx := c.Request().Context()
	total, err := h.svc.Weddings.StorageTotal(ctx)
	if err != nil {
		return err
	}
	top, err := h.svc.Weddings.AdminList(ctx, wedding.AdminFilter{Sort: "storage"}, 20)
	if err != nil {
		return err
	}
	usage := make([]gallery.Usage, len(top.Weddings))
	for i, w := range top.Weddings {
		if usage[i], err = h.svc.Gallery.StorageUsage(ctx, w.ID); err != nil {
			return err
		}
	}
	return web.Render(c, http.StatusOK, storagePage(total, top.Total, top.Weddings, usage))
}

func (h *Handler) Domains(c echo.Context) error {
	ctx := c.Request().Context()
	page := pageParam(c)
	ds, total, err := h.svc.Domains.ListAll(ctx, page, PerPage)
	if err != nil {
		return err
	}
	active, err := h.svc.Domains.ActiveCount(ctx)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, domainsPage(ds, total, page, active, h.svc.Domains.QuotaWarn()))
}

func (h *Handler) Audit(c echo.Context) error {
	page := pageParam(c)
	es, total, err := h.svc.AuditLog(c.Request().Context(), "", page)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, auditPage(es, total, page))
}
