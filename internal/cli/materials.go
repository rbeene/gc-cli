package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	gclassroom "google.golang.org/api/classroom/v1"

	"github.com/timothy/gc-cli/internal/auth"
	"github.com/timothy/gc-cli/internal/classroom"
	"github.com/timothy/gc-cli/internal/drive"
)

// Attachment kinds, as reported by materials list/fetch.
const (
	kindDriveFile = "drive_file"
	kindLink      = "link"
	kindForm      = "form"
	kindYouTube   = "youtube"
)

// attachment is one material hanging off one assignment, flattened so the
// course work it belongs to travels with it.
type attachment struct {
	CourseWorkID    string `json:"course_work_id"`
	CourseWorkTitle string `json:"course_work_title"`
	Kind            string `json:"kind"`
	Title           string `json:"title"`
	FileID          string `json:"file_id,omitempty"`
	URL             string `json:"url,omitempty"`
	Downloadable    bool   `json:"downloadable"`
}

func newMaterialsCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "materials",
		Aliases: []string{"attachments", "mat"},
		Short:   "List and download assignment attachments",
		Long: "Read the files attached to assignments and download them locally.\n\n" +
			"Downloading requires the drive.readonly scope, which is not part of the\n" +
			"default login: a teacher's attachment lives in the teacher's Drive and is\n" +
			"only shared with this account, so the narrower drive.file scope cannot\n" +
			"reach it.",
		Example: "  gc materials list -c <course_id>\n  gc materials fetch -c <course_id> --out ~/homework\n  gc materials fetch -c <course_id> -w <course_work_id> --out ~/homework --json",
	}
	cmd.AddCommand(
		newMaterialsListCmd(app),
		newMaterialsFetchCmd(app),
	)
	return cmd
}

func newMaterialsListCmd(app *App) *cobra.Command {
	var courseID, courseWorkID string
	var pageSize int64
	cmd := &cobra.Command{
		Use:     "list [course_id]",
		Aliases: []string{"ls"},
		Short:   "List attachments on assignments",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fillPositional(args, &courseID)
			if err := requireValue(courseID, "course", "course_id"); err != nil {
				return err
			}
			ctx := ctx(cmd)
			client, err := app.ClassroomClient(ctx, []string{auth.ScopeCourseWorkMeReadonly})
			if err != nil {
				return err
			}
			items, err := collectCourseWork(ctx, client, courseID, courseWorkID, pageSize)
			if err != nil {
				return err
			}
			found := flattenAttachments(items)
			if app.Printer.JSON {
				return app.Printer.PrintJSON(map[string]any{"attachments": found})
			}
			for _, a := range found {
				target := a.FileID
				if target == "" {
					target = a.URL
				}
				_, _ = fmt.Fprintf(app.Out, "%s\t%s\t%s\t%s\t%s\n", a.CourseWorkID, a.CourseWorkTitle, a.Kind, a.Title, target)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&courseID, "course", "c", "", "Course ID")
	cmd.Flags().StringVarP(&courseWorkID, "course-work", "w", "", "Limit to one CourseWork ID")
	cmd.Flags().Int64VarP(&pageSize, "page-size", "n", 50, "Number of assignments to scan")
	return cmd
}

func newMaterialsFetchCmd(app *App) *cobra.Command {
	var courseID, courseWorkID, outDir string
	var pageSize int64
	cmd := &cobra.Command{
		Use:     "fetch [course_id]",
		Aliases: []string{"download", "dl"},
		Short:   "Download assignment attachments to a local directory",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fillPositional(args, &courseID)
			if err := requireValue(courseID, "course", "course_id"); err != nil {
				return err
			}
			if err := requireValue(outDir, "out", "output directory"); err != nil {
				return err
			}
			ctx := ctx(cmd)
			client, err := app.ClassroomClient(ctx, []string{auth.ScopeCourseWorkMeReadonly})
			if err != nil {
				return err
			}
			items, err := collectCourseWork(ctx, client, courseID, courseWorkID, pageSize)
			if err != nil {
				return err
			}
			found := flattenAttachments(items)

			downloadable := make([]attachment, 0, len(found))
			skipped := make([]map[string]string, 0)
			for _, a := range found {
				if a.Downloadable {
					downloadable = append(downloadable, a)
					continue
				}
				skipped = append(skipped, map[string]string{
					"course_work_id": a.CourseWorkID,
					"kind":           a.Kind,
					"title":          a.Title,
					"url":            a.URL,
					"reason":         "not a Drive file; nothing to download",
				})
			}

			results := make([]*drive.DownloadResult, 0, len(downloadable))
			if len(downloadable) > 0 {
				driveClient, err := app.DriveClient(ctx, []string{auth.ScopeDriveReadonly})
				if err != nil {
					return err
				}
				for _, a := range downloadable {
					dest := filepath.Join(outDir, drive.SafeFileName(a.CourseWorkTitle, a.CourseWorkID))
					res, err := driveClient.DownloadFile(ctx, a.FileID, dest)
					if err != nil {
						// One unreachable attachment must not sink the rest: a
						// quiz Form or a file the teacher later unshared is
						// reported and skipped.
						if errors.Is(err, drive.ErrNotDownloadable) {
							skipped = append(skipped, map[string]string{
								"course_work_id": a.CourseWorkID,
								"kind":           a.Kind,
								"title":          a.Title,
								"file_id":        a.FileID,
								"reason":         err.Error(),
							})
							continue
						}
						return err
					}
					results = append(results, res)
				}
			}

			if app.Printer.JSON {
				return app.Printer.PrintJSON(map[string]any{"downloaded": results, "skipped": skipped})
			}
			for _, r := range results {
				note := ""
				if r.Exported {
					note = "\t(exported)"
				}
				_, _ = fmt.Fprintf(app.Out, "%s\t%d bytes\t%s%s\n", r.Path, r.Bytes, r.MimeType, note)
			}
			for _, s := range skipped {
				_, _ = fmt.Fprintf(app.Out, "skipped\t%s\t%s\n", s["title"], s["reason"])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&courseID, "course", "c", "", "Course ID")
	cmd.Flags().StringVarP(&courseWorkID, "course-work", "w", "", "Limit to one CourseWork ID")
	cmd.Flags().StringVarP(&outDir, "out", "o", "", "Directory to write attachments into")
	cmd.Flags().Int64VarP(&pageSize, "page-size", "n", 50, "Number of assignments to scan")
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

// collectCourseWork returns either the single requested assignment or the
// course's assignment list, so list and fetch share one code path.
func collectCourseWork(ctx context.Context, client classroom.ClassroomClient, courseID, courseWorkID string, pageSize int64) ([]*gclassroom.CourseWork, error) {
	if courseWorkID != "" {
		item, err := client.GetCourseWork(ctx, courseID, courseWorkID)
		if err != nil {
			return nil, err
		}
		return []*gclassroom.CourseWork{item}, nil
	}
	items, _, err := client.ListCourseWork(ctx, courseID, classroom.ListParams{PageSize: pageSize})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// flattenAttachments turns the nested material shapes the Classroom API returns
// into one flat list, marking which entries actually have bytes behind them.
func flattenAttachments(items []*gclassroom.CourseWork) []attachment {
	out := make([]attachment, 0)
	for _, item := range items {
		if item == nil {
			continue
		}
		for _, m := range item.Materials {
			if m == nil {
				continue
			}
			a := attachment{CourseWorkID: item.Id, CourseWorkTitle: item.Title}
			switch {
			case m.DriveFile != nil && m.DriveFile.DriveFile != nil:
				a.Kind = kindDriveFile
				a.Title = m.DriveFile.DriveFile.Title
				a.FileID = m.DriveFile.DriveFile.Id
				a.URL = m.DriveFile.DriveFile.AlternateLink
				a.Downloadable = a.FileID != ""
			case m.Form != nil:
				a.Kind = kindForm
				a.Title = m.Form.Title
				a.URL = m.Form.FormUrl
			case m.YoutubeVideo != nil:
				a.Kind = kindYouTube
				a.Title = m.YoutubeVideo.Title
				a.URL = m.YoutubeVideo.AlternateLink
			case m.Link != nil:
				a.Kind = kindLink
				a.Title = m.Link.Title
				a.URL = m.Link.Url
			default:
				continue
			}
			if a.Title == "" {
				a.Title = a.Kind
			}
			out = append(out, a)
		}
	}
	return out
}
