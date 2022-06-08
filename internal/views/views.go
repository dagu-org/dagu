package views

import (
	"fmt"

	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/utils"
)

func GetViews() []*models.View {
	s := storage.NewStorage(
		settings.MustGet(settings.CONFIG__VIEWS_DIR),
	)
	fis, err := s.List()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	ret := make([]*models.View, 0, len(fis))
	for _, fi := range fis {
		dat := s.MustRead(fi.Name())
		if dat != nil {
			v, err := models.ViewFromJson(dat)
			utils.LogIgnoreErr("Controller: get views", err)
			if err == nil {
				ret = append(ret, v)
			}
		}
	}
	return ret
}

func SaveView(view *models.View) error {
	s := storage.NewStorage(
		settings.MustGet(settings.CONFIG__VIEWS_DIR),
	)
	if view.Name == "" {
		return ErrInvalidName
	}
	b, err := view.ToJson()
	if err != nil {
		return err
	}
	return s.Save(fmt.Sprintf("%s.json", view.Name), b)
}

func DeleteView(view *models.View) error {
	s := storage.NewStorage(
		settings.MustGet(settings.CONFIG__VIEWS_DIR),
	)
	return s.Delete(fmt.Sprintf("%s.json", view.Name))
}

var ErrInvalidName = fmt.Errorf("view's name is invalid or empty")
var ErrNotFound = fmt.Errorf("not found")

func GetView(name string) (*models.View, error) {
	s := storage.NewStorage(
		settings.MustGet(settings.CONFIG__VIEWS_DIR),
	)
	dat := s.MustRead(fmt.Sprintf("%s.json", name))
	if dat == nil {
		return nil, ErrNotFound
	}
	return models.ViewFromJson(dat)
}
