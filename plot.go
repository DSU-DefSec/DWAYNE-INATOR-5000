package main

import (
	"image/color"
	"math"
	"math/rand"
	"os"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

func graphScores(records []TeamRecord) {
	rand.Seed(int64(time.Now().Nanosecond()))

	p := plot.New()

	p.X.Label.Text = "Round"
	p.X.Label.TextStyle.Color = color.Black
	p.X.Color = color.Black
	p.X.Width = 2
	p.X.Tick.Color = color.Black
	p.X.Tick.Label.Color = color.Black
	p.Y.Label.Text = "Score"
	p.Y.Label.TextStyle.Color = color.Black
	p.Y.Color = color.Black
	p.Y.Tick.Color = color.Black
	p.Y.Tick.Label.Color = color.Black
	p.Y.Width = 2
	p.BackgroundColor = color.Transparent
	p.BackgroundColor = color.Transparent
	p.Legend.TextStyle.Color = color.Black

	graphData := make([]interface{}, len(records)*2)

	for i, rec := range records {
		graphData[i*2] = rec.Team.Name
		l, _ := plotter.NewLine(getTeamPoints(rec.Team.ID))
		l.LineStyle.Width = vg.Points(1)
		l.LineStyle.Color = color.RGBA{R: uint8(int(math.Pow(255, float64(i+3)))%80), B: uint8(int(math.Pow(255, float64(i+5)))%70), G: uint8(int(math.Pow(255, float64(i+7)))%90), A: 255}
		p.Add(l)
		p.Legend.Add(rec.Team.Name, l)
	}

	c := vgimg.PngCanvas{vgimg.NewWith(
		vgimg.UseWH(25*vg.Centimeter, 12*vg.Centimeter),
		vgimg.UseBackgroundColor(color.Transparent),
	)}
	p.Draw(draw.New(c))

	// Save the plot to a png file
	f, err := os.Create("assets/points.png")
	if err != nil {
		errorPrint(err)
		return
	}
	defer f.Close()

	_, err = c.WriteTo(f)
	if err != nil {
		errorPrint(err)
	}
}

func getTeamPoints(teamID uint) plotter.XYs {
	var records []TeamRecord
	res := db.Where("team_id = ?", teamID).Order("time asc").Find(&records)
	if res.Error != nil {
		errorPrint(res.Error)
		return nil
	}
	pts := make(plotter.XYs, len(records))
	for i := range pts {
		pts[i].X = float64(records[i].Round)
		pts[i].Y = float64(calculateScoreTotal(records[i]))
	}
	return pts
}
