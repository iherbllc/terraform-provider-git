package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/iherbllc/terraform-provider-git/internal/git"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
)

func ResourceGitFile() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitFileCreate,
		ReadContext:   resourceGitFileRead,
		UpdateContext: resourceGitFileUpdate,
		DeleteContext: resourceGitFileDelete,
		Schema: map[string]*schema.Schema{
			"repository": {
				Type:     schema.TypeString,
				Required: true,
			},
			"folder": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"file_name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"path": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"content": {
				Type:     schema.TypeString,
				Required: true,
			},
			"branch": {
				Type:     schema.TypeString,
				Required: true,
			},
			"author": {
				Type:     schema.TypeString,
				Required: true,
			},
			"email": {
				Type:     schema.TypeString,
				Required: true,
			},
			"postfix": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceGitFileCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	folder, file := resolvePath(d)

	fileName := fmt.Sprintf("%s/%s", folder, file)
	d.SetId(fileName)

	g := m.(*git.Client)

	w := git.WriteRequest{
		ReadRequest: git.ReadRequest{
			Repository: d.Get("repository").(string),
			Branch:     d.Get("branch").(string),
			Path:       folder,
			FileName:   file,
		},
		Content: d.Get("content").(string),
		Name:    d.Get("author").(string),
		Email:   d.Get("email").(string),
		Postfix: d.Get("postfix").(string),
	}
	_, err := g.Write(ctx, w)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "failed to write file"))
	}

	resourceGitFileRead(ctx, d, m)

	return diags
}

func resourceGitFileRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	folder, file := resolvePath(d)

	g := m.(*git.Client)

	c, err := g.Read(ctx, git.ReadRequest{
		Repository: d.Get("repository").(string),
		Branch:     d.Get("branch").(string),
		Path:       folder,
		FileName:   file,
	})
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "failed to read file"))
	}

	if c.Exists {
		err = d.Set("content", string(c.Contents))
		if err != nil {
			return diag.FromErr(errors.Wrap(err, "failed to set content"))
		}
	} else {
		d.Set("content", "")
	}

	return diags
}

func resourceGitFileUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	g := m.(*git.Client)

	folder, file := resolvePath(d)

	w := git.WriteRequest{
		ReadRequest: git.ReadRequest{
			Repository: d.Get("repository").(string),
			Branch:     d.Get("branch").(string),
			Path:       folder,
			FileName:   file,
		},
		Content: d.Get("content").(string),
		Name:    d.Get("author").(string),
		Email:   d.Get("email").(string),
		Postfix: d.Get("postfix").(string),
	}
	_, err := g.Write(ctx, w)
	if err != nil {
		return diag.FromErr(errors.Wrap(err, "failed to write file"))
	}

	return resourceGitFileRead(ctx, d, m)
}

func resourceGitFileDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

func resolvePath(d *schema.ResourceData) (string, string) {
	path := d.Get("path").(string)
	if path == "" {
		return d.Get("folder").(string), d.Get("file_name").(string)
	}

	frags := strings.Split(path, "/")

	if len(frags) > 1 {
		folder := strings.Join(frags[:len(frags)-1], "/")
		file := frags[len(frags)-1]
		return folder, file
	} else {
		return "", frags[0]
	}
}
