package main

import (
	"fmt"
	"math/rand"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func graphScores(records []teamRecord) {
	if mewConf.Tightlipped {
		fmt.Println("graphScores: can't create image, tightlipped mode enabled")
		return
	}

	rand.Seed(int64(time.Now().Nanosecond()))

	p, err := plot.New()
	if err != nil {
		fmt.Println("plot:", err)
		return
	}

	p.X.Label.Text = "Time"
	p.Y.Label.Text = "Score"
	graphData := make([]interface{}, len(records)*2)

	for i, rec := range records {
		graphData[i*2] = rec.Team.Name
		graphData[(i*2)+1] = getTeamPoints(rec.Team)
		if err != nil {
			fmt.Println("plot:", err)
			return
		}
	}

	err = plotutil.AddLinePoints(p,
		graphData...)
	if err != nil {
		fmt.Println("plot:", err)
		return
	}

	// Save the plot to a PNG file.
	if err := p.Save(7*vg.Inch, 4*vg.Inch, "assets/points.png"); err != nil {
		fmt.Println("plot:", err)
		return
	}
}

// randomPoints returns some random x, y points.
func getTeamPoints(team teamData) plotter.XYs {
	records, err := getTeamRecords(team, 0)
	pts := make(plotter.XYs, len(records))
	if err != nil {
		fmt.Println("getTeamPoints:", err)
		return pts
	}
	for i := range pts {
		pts[i].X = float64(records[i].Round)
		pts[i].Y = float64(records[i].Total)
	}
	return pts
}
