package admin

import (
	"encoding/json"
	"fmt"

	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/utils"
)

type View struct {
	Name        string
	Desc        string
	ContainTags []string
}

func ViewFromJson(b []byte) (*View, error) {
	v := &View{}
	err := json.Unmarshal(b, v)
	return v, err
}

func (v *View) ToJson() ([]byte, error) {
	return json.Marshal(v)
}

func GetViews() []*View {
	s := storage.NewStorage(
		settings.MustGet(settings.SETTING__VIEWS_DIR),
	)
	fis, err := s.List()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	ret := make([]*View, 0, len(fis))
	for _, fi := range fis {
		dat := s.MustRead(fi.Name())
		if dat != nil {
			v, err := ViewFromJson(dat)
			utils.LogErr("Controller: get views", err)
			if err == nil {
				ret = append(ret, v)
			}
		}
	}
	return ret
}

func SaveView(view *View) error {
	s := storage.NewStorage(
		settings.MustGet(settings.SETTING__VIEWS_DIR),
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

func DeleteView(view *View) error {
	s := storage.NewStorage(
		settings.MustGet(settings.SETTING__VIEWS_DIR),
	)
	return s.Delete(fmt.Sprintf("%s.json", view.Name))
}

var ErrInvalidName = fmt.Errorf("view's name is invalid or empty")
var ErrNotFound = fmt.Errorf("not found")

func GetView(name string) (*View, error) {
	s := storage.NewStorage(
		settings.MustGet(settings.SETTING__VIEWS_DIR),
	)
	dat := s.MustRead(fmt.Sprintf("%s.json", name))
	if dat == nil {
		return nil, ErrNotFound
	}
	return ViewFromJson(dat)
}
