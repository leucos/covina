package grafana

type SearchRequest struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

type QueryRequest struct {
	PanelID int `json:"panelId"`
	Range   struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
		Raw  struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"raw"`
	} `json:"range"`
	RangeRaw struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"rangeRaw"`
	Interval      string `json:"interval"`
	IntervalMs    int    `json:"intervalMs"`
	MaxDataPoints int    `json:"maxDataPoints"`
	Targets       []struct {
		Target string `json:"target"`
		RefID  string `json:"refId"`
		Type   string `json:"type"`
		Data   struct {
			Additional string `json:"additional"`
		} `json:"data,omitempty"`
	} `json:"targets"`
	AdhocFilters []struct {
		Key      string `json:"key"`
		Operator string `json:"operator"`
		Value    string `json:"value"`
	} `json:"adhocFilters"`
}

type AnnotationsRequest struct {
	Range struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"range"`
	RangeRaw struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"rangeRaw"`
	Annotation struct {
		Name       string `json:"name"`
		Datasource string `json:"datasource"`
		IconColor  string `json:"iconColor"`
		Enable     bool   `json:"enable"`
		Query      string `json:"query"`
	} `json:"annotation"`
	Variables []string `json:"variables"`
}

type TagKeysRequest struct{}

type TagValuesRequest struct {
	Key string `json:"key"`
}

type SearchResponse []string

type QueryTimeseriesResponse []struct {
	Target     string          `json:"target"`
	Datapoints [][]interface{} `json:"datapoints"`
}

type QueryTableResponse struct {
	Columns []struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"columns"`
	Rows [][]interface{} `json:"rows"`
	Type string          `json:"type"`
}

type AnnotationsResponse []struct {
	Text     string   `json:"text"`
	Title    string   `json:"title"`
	IsRegion bool     `json:"isRegion"`
	Time     string   `json:"time"`
	TimeEnd  string   `json:"timeEnd"`
	Tags     []string `json:"tags"`
}

type TagKeysReponse []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type TagValuesResponse []struct {
	Text string `json:"text"`
}
