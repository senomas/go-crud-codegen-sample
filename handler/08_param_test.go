package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"example.com/app-api/model"
	"example.com/app-api/util"
	_ "example.com/app-api/util"
	"example.com/app-api/util/jsql"
)

func TestParamCrudApi(t *testing.T) {
	store := model.GetStore()
	ctx := context.Background()

	t.Run("Create param test_param_1", func(t *testing.T) {
		param := model.Param{
			Group:       "GENERAL",
			Code:        "test_param_1",
			Value:       jsql.NullStringValue("param_value_1"),
			Description: jsql.NullStringValue("Parameter untuk pengujian create"),
		}

		res, err := store.Param().Create(ctx, param)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, int64(1), res.ID)
	})

	// --- Create via API ---
	t.Run("Create param test_param_2", func(t *testing.T) {
		param := model.Param{
			Group:       "GENERAL",
			Code:        "test_param_2",
			Value:       jsql.NullStringValue("value_2"),
			Description: jsql.NullStringValue("parameter untuk testing kedua"),
		}

		body, _ := json.Marshal(param)

		req, err := http.NewRequest(http.MethodPut, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
		assert.NoError(t, err)
		util.SetHMAC(req, body, shared)

		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session", Value: token})

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		bb, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode, fmt.Sprintf("response: %s", string(bb)))

		err = json.Unmarshal(bb, &param)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), param.ID)
	})

	/* t.Run("Create param with duplicate group and code", func(t *testing.T) {
		param := model.Param{
			Group:       "GENERAL",
			Code:        "test_param_2", // sama dengan test sebelumnya → duplikat
			Value:       jsql.NullStringValue("value_duplicated"),
			Description: jsql.NullStringValue("parameter duplikat untuk testing"),
		}

		body, _ := json.Marshal(param)

		req, err := http.NewRequest(http.MethodPut, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
		assert.NoError(t, err)

		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session", Value: token})

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		bb, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)

		t.Logf("Response: %s", string(bb))

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "harusnya error 500 untuk duplikat")

		var respData map[string]interface{}
		err = json.Unmarshal(bb, &respData)
		assert.NoError(t, err)

		assert.Equal(t, "error", respData["code"])
		assert.Contains(t, fmt.Sprintf("%v", respData["error"]), "SQLSTATE=23505", "harus mengandung SQLSTATE=23505 untuk duplikat constraint")
	}) */

	// --- Create dummy data langsung ke DB ---
	/* t.Run("Create 21 dummy param", func(t *testing.T) {
		time.Sleep(2 * time.Second) // pastikan timestamp berbeda
		tbeforeDummy = time.Now()
		time.Sleep(2 * time.Second)
		ct := time.Now()
		for i := 1; i <= 21; i++ {
			param := model.Param{
				Group:       fmt.Sprintf("GROUP_%d", i),
				Code:        fmt.Sprintf("dummy_param_%d", i),
				Value:       jsql.NullStringValue(fmt.Sprintf("value_%d", i)),
				Description: jsql.NullStringValue(fmt.Sprintf("description dummy %d", i)),
				UpdatedAt:   ct,
			}

			res, err := store.Param().Create(ctx, param)
			assert.NoError(t, err)
			assert.NotNil(t, res)
			assert.Equal(t, int64(3+i), res.ID)
		}
		time.Sleep(2 * time.Second)
		tafterDummy = time.Now()
		time.Sleep(2 * time.Second)
	}) */

	// t.Run("Find param eq dummy_param_1", func(t *testing.T) {
	// 	bv, _ := json.Marshal("dummy_param_1")
	// 	body, _ := json.Marshal(struct {
	// 		Limit   int                  `json:"limit"`
	// 		Offset  int64                `json:"offset,omitempty"`
	// 		Filter  []model.ParamFilter  `json:"filter,omitempty"`
	// 		Sorting []model.ParamSorting `json:"sorting,omitempty"`
	// 	}{
	// 		Limit:  5,
	// 		Offset: 0,
	// 		Filter: []model.ParamFilter{{
	// 			Field: model.ParamField_Code,
	// 			Op:    model.FilterOp_EQ,
	// 			Value: json.RawMessage(bv),
	// 		}},
	// 		Sorting: []model.ParamSorting{{
	// 			Field: model.ParamField_Code,
	// 			Dir:   model.SortDir_DESC,
	// 		}},
	// 	})

	// 	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
	// 	assert.NoError(t, err)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})

	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)

	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	var result struct {
	// 		List  []model.Param `json:"list"`
	// 		Total int64         `json:"total"`
	// 	}
	// 	err = json.Unmarshal(bb, &result)
	// 	assert.NoError(t, err)

	// 	assert.Equal(t, int64(1), result.Total, "total must match")
	// 	assert.Equal(t, 1, len(result.List), "len(params) must match")
	// })

	// t.Run("Find all param", func(t *testing.T) {
	// 	body, _ := json.Marshal(struct {
	// 		Limit   int                  `json:"limit"`
	// 		Offset  int64                `json:"offset,omitempty"`
	// 		Filter  []model.ParamFilter  `json:"filter,omitempty"`
	// 		Sorting []model.ParamSorting `json:"sorting,omitempty"`
	// 	}{
	// 		Sorting: []model.ParamSorting{{
	// 			Field: model.ParamField_ID,
	// 			Dir:   model.SortDir_ASC,
	// 		}},
	// 		Limit: 10,
	// 	})

	// 	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
	// 	assert.NoError(t, err)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})

	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)

	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	var result struct {
	// 		List  []model.Param `json:"list"`
	// 		Total int64         `json:"total"`
	// 	}
	// 	err = json.Unmarshal(bb, &result)
	// 	assert.NoError(t, err)

	// 	assert.Equal(t, int64(23), result.Total)
	// 	assert.Equal(t, 10, len(result.List))

	// 	for i, u := range []string{"test_param_1", "test_param_2", "dummy_param_1", "dummy_param_2", "dummy_param_3"} {
	// 		assert.Equal(t, u, result.List[i].Code, fmt.Sprintf("code must match at index %d", i))
	// 	}
	// })

	// t.Run("Find all param limit 5 offset 5", func(t *testing.T) {
	// 	body, _ := json.Marshal(struct {
	// 		Limit   int                  `json:"limit"`
	// 		Offset  int64                `json:"offset,omitempty"`
	// 		Filter  []model.ParamFilter  `json:"filter,omitempty"`
	// 		Sorting []model.ParamSorting `json:"sorting,omitempty"`
	// 	}{
	// 		Limit:  5,
	// 		Offset: 5,
	// 	})

	// 	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
	// 	assert.NoError(t, err)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)
	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	var result struct {
	// 		List  []model.Param `json:"list"`
	// 		Total int64         `json:"total"`
	// 	}
	// 	err = json.Unmarshal(bb, &result)
	// 	assert.NoError(t, err)

	// 	assert.Equal(t, int64(23), result.Total)
	// 	assert.Equal(t, 5, len(result.List), "len(params) must match")

	// 	for i, u := range []string{"dummy_param_4", "dummy_param_5", "dummy_param_6"} {
	// 		assert.Equal(t, u, result.List[i].Code, fmt.Sprintf("code must match at index %d", i))
	// 	}
	// })

	// t.Run("Find params code like %dummy% limit 5", func(t *testing.T) {
	// 	bv, _ := json.Marshal("%dummy%")
	// 	body, _ := json.Marshal(struct {
	// 		Limit   int                  `json:"limit"`
	// 		Offset  int64                `json:"offset,omitempty"`
	// 		Filter  []model.ParamFilter  `json:"filter,omitempty"`
	// 		Sorting []model.ParamSorting `json:"sorting,omitempty"`
	// 	}{
	// 		Limit:  5,
	// 		Offset: 0,
	// 		Filter: []model.ParamFilter{{
	// 			Field: model.ParamField_Code,
	// 			Op:    model.FilterOp_Like,
	// 			Value: json.RawMessage(bv),
	// 		}},
	// 		Sorting: []model.ParamSorting{{
	// 			Field: model.ParamField_Code,
	// 			Dir:   model.SortDir_ASC,
	// 		}},
	// 	})

	// 	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
	// 	assert.NoError(t, err)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)
	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	var result struct {
	// 		List  []model.Param `json:"list"`
	// 		Total int64         `json:"total"`
	// 	}
	// 	err = json.Unmarshal(bb, &result)
	// 	assert.NoError(t, err)

	// 	assert.Equal(t, int64(21), result.Total, "total must match")
	// 	assert.Equal(t, 5, len(result.List), "len(params) must match")

	// 	expectedCodes := []string{"dummy_param_1", "dummy_param_10", "dummy_param_11", "dummy_param_12", "dummy_param_13"}
	// 	for i, expectedCode := range expectedCodes {
	// 		assert.Equal(t, expectedCode, result.List[i].Code, fmt.Sprintf("code must match at index %d", i))
	// 	}
	// })

	// // --- BLOK TES DUPLIKAT YANG SALAH TELAH DIHAPUS DARI SINI ---

	// t.Run("Find params code like DuMMy% limit 5", func(t *testing.T) {
	// 	bv, _ := json.Marshal("%DuMMy%")
	// 	body, _ := json.Marshal(struct {
	// 		Limit   int                  `json:"limit"`
	// 		Offset  int64                `json:"offset,omitempty"`
	// 		Filter  []model.ParamFilter  `json:"filter,omitempty"`
	// 		Sorting []model.ParamSorting `json:"sorting,omitempty"`
	// 	}{
	// 		Limit:  5,
	// 		Offset: 0,
	// 		Filter: []model.ParamFilter{{
	// 			Field: model.ParamField_Code,
	// 			Op:    model.FilterOp_Like,
	// 			Value: json.RawMessage(bv),
	// 		}},
	// 	})

	// 	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/v1/param", bytes.NewReader(body))
	// 	assert.NoError(t, err)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)
	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	var result struct {
	// 		List  []model.Param `json:"list"`
	// 		Total int64         `json:"total"`
	// 	}
	// 	err = json.Unmarshal(bb, &result)
	// 	assert.NoError(t, err)

	// 	assert.Equal(t, int64(0), result.Total, "total must match")
	// 	assert.Equal(t, 0, len(result.List), "len(params) must match")
	// 	/* for i, u := range []string{"param_dummy_1", "param_dummy_2", "param_dummy_3", "param_dummy_4"} {
	// 		assert.Equal(t, u, result.List[i].Code, fmt.Sprintf("code must match at index %d", i))
	// 	} */
	// })

	// t.Run("Update param param_act_1", func(t *testing.T) {
	// 	param := model.Param{
	// 		Code:  "param_act_3",
	// 		Value: jsql.NullStringValue("1000275"),
	// 	}
	// 	body, _ := json.Marshal(struct {
	// 		Value  model.Param `json:"value"`
	// 		Fields []string    `json:"fields"`
	// 	}{
	// 		Value:  param,
	// 		Fields: []string{"code", "value"},
	// 	})

	// 	req, err := http.NewRequest(http.MethodPatch, "http://localhost:8080/api/v1/param/1", bytes.NewReader(body))
	// 	assert.NoError(t, err)
	// 	util.SetHMAC(req, body, shared)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)
	// 	bb, err := io.ReadAll(resp.Body)
	// 	assert.NoError(t, err)

	// 	err = json.Unmarshal(bb, &param)
	// 	assert.NoError(t, err)
	// 	assert.Equal(t, int64(1), param.ID)

	// 	assert.Equal(t, "param_act_3", param.Code)
	// })

	// t.Run("Delete param param_dummy_1", func(t *testing.T) {
	// 	req, err := http.NewRequest(http.MethodDelete, "http://localhost:8080/api/v1/param/1", nil)
	// 	assert.NoError(t, err)
	// 	util.SetHMAC(req, nil, shared)

	// 	req.Header.Set("Content-Type", "application/json")
	// 	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	// 	resp, err := http.DefaultClient.Do(req)
	// 	assert.NoError(t, err)
	// 	defer resp.Body.Close()

	// 	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// 	assert.NotNil(t, resp.Body)
	// })
}
