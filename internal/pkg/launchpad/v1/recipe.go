// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// --- Rock Recipes ---

// GetRockRecipe fetches a rock recipe.
// Path: /~<owner>/<project>/+rock/<name>
func (c *Client) GetRockRecipe(ctx context.Context, owner, project, name string) (RockRecipe, error) {
	var r RockRecipe
	path := fmt.Sprintf("/~%s/%s/+rock/%s", owner, project, name)
	if err := c.GetJSON(ctx, path, &r); err != nil {
		return RockRecipe{}, fmt.Errorf("fetching rock recipe: %w", err)
	}
	return r, nil
}

// GetRockRecipeByLink fetches a rock recipe using its self_link.
func (c *Client) GetRockRecipeByLink(ctx context.Context, selfLink string) (RockRecipe, error) {
	var r RockRecipe
	if err := c.GetJSON(ctx, selfLink, &r); err != nil {
		return RockRecipe{}, fmt.Errorf("fetching rock recipe: %w", err)
	}
	return r, nil
}

// RequestRockRecipeBuilds requests builds for a rock recipe.
func (c *Client) RequestRockRecipeBuilds(ctx context.Context, recipeSelfLink string, channels map[string]string, architectures []string) (BuildRequest, error) {
	form := url.Values{
		"ws.op": {"requestBuilds"},
	}
	if len(channels) > 0 {
		ch, _ := json.Marshal(channels)
		form.Set("channels", string(ch))
	}
	if len(architectures) > 0 {
		form.Set("architectures", strings.Join(architectures, ","))
	}
	var br BuildRequest
	if err := c.PostJSON(ctx, recipeSelfLink, form, &br); err != nil {
		return BuildRequest{}, fmt.Errorf("requesting rock builds: %w", err)
	}
	return br, nil
}

// GetRockRecipeBuilds returns all builds for a rock recipe.
func (c *Client) GetRockRecipeBuilds(ctx context.Context, recipeSelfLink string) ([]RockRecipeBuild, error) {
	return GetAllPages[RockRecipeBuild](ctx, c, recipeSelfLink+"/builds")
}

// GetRockRecipeBuild fetches a specific rock recipe build by self_link.
func (c *Client) GetRockRecipeBuild(ctx context.Context, selfLink string) (RockRecipeBuild, error) {
	var b RockRecipeBuild
	if err := c.GetJSON(ctx, selfLink, &b); err != nil {
		return RockRecipeBuild{}, fmt.Errorf("fetching rock build: %w", err)
	}
	return b, nil
}

// RetryRockRecipeBuild retries a failed rock recipe build.
func (c *Client) RetryRockRecipeBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"retry"}})
	return err
}

// CancelRockRecipeBuild cancels a pending or in-progress rock recipe build.
func (c *Client) CancelRockRecipeBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"cancel"}})
	return err
}

// GetRockRecipeBuildFileURLs returns URLs for files produced by a build.
func (c *Client) GetRockRecipeBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error) {
	u := wsOpURL(buildSelfLink, "getFileUrls", nil)
	var urls []string
	if err := c.GetJSON(ctx, u, &urls); err != nil {
		return nil, fmt.Errorf("fetching build file URLs: %w", err)
	}
	return urls, nil
}

// --- Charm Recipes ---

// GetCharmRecipe fetches a charm recipe.
// Path: /~<owner>/<project>/+charm/<name>
func (c *Client) GetCharmRecipe(ctx context.Context, owner, project, name string) (CharmRecipe, error) {
	var r CharmRecipe
	path := fmt.Sprintf("/~%s/%s/+charm/%s", owner, project, name)
	if err := c.GetJSON(ctx, path, &r); err != nil {
		return CharmRecipe{}, fmt.Errorf("fetching charm recipe: %w", err)
	}
	return r, nil
}

// GetCharmRecipeByLink fetches a charm recipe using its self_link.
func (c *Client) GetCharmRecipeByLink(ctx context.Context, selfLink string) (CharmRecipe, error) {
	var r CharmRecipe
	if err := c.GetJSON(ctx, selfLink, &r); err != nil {
		return CharmRecipe{}, fmt.Errorf("fetching charm recipe: %w", err)
	}
	return r, nil
}

// RequestCharmRecipeBuilds requests builds for a charm recipe.
func (c *Client) RequestCharmRecipeBuilds(ctx context.Context, recipeSelfLink string, channels map[string]string, architectures []string) (BuildRequest, error) {
	form := url.Values{
		"ws.op": {"requestBuilds"},
	}
	if len(channels) > 0 {
		ch, _ := json.Marshal(channels)
		form.Set("channels", string(ch))
	}
	if len(architectures) > 0 {
		form.Set("architectures", strings.Join(architectures, ","))
	}
	var br BuildRequest
	if err := c.PostJSON(ctx, recipeSelfLink, form, &br); err != nil {
		return BuildRequest{}, fmt.Errorf("requesting charm builds: %w", err)
	}
	return br, nil
}

// GetCharmRecipeBuilds returns all builds for a charm recipe.
func (c *Client) GetCharmRecipeBuilds(ctx context.Context, recipeSelfLink string) ([]CharmRecipeBuild, error) {
	return GetAllPages[CharmRecipeBuild](ctx, c, recipeSelfLink+"/builds")
}

// GetCharmRecipeBuild fetches a specific charm recipe build.
func (c *Client) GetCharmRecipeBuild(ctx context.Context, selfLink string) (CharmRecipeBuild, error) {
	var b CharmRecipeBuild
	if err := c.GetJSON(ctx, selfLink, &b); err != nil {
		return CharmRecipeBuild{}, fmt.Errorf("fetching charm build: %w", err)
	}
	return b, nil
}

// RetryCharmRecipeBuild retries a failed charm recipe build.
func (c *Client) RetryCharmRecipeBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"retry"}})
	return err
}

// CancelCharmRecipeBuild cancels a pending or in-progress charm recipe build.
func (c *Client) CancelCharmRecipeBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"cancel"}})
	return err
}

// ScheduleCharmBuildStoreUpload schedules a store upload for a charm build.
func (c *Client) ScheduleCharmBuildStoreUpload(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"scheduleStoreUpload"}})
	return err
}

// GetCharmRecipeBuildFileURLs returns URLs for files produced by a charm build.
func (c *Client) GetCharmRecipeBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error) {
	u := wsOpURL(buildSelfLink, "getFileUrls", nil)
	var urls []string
	if err := c.GetJSON(ctx, u, &urls); err != nil {
		return nil, fmt.Errorf("fetching build file URLs: %w", err)
	}
	return urls, nil
}

// --- Snap Packages ---

// GetSnap fetches a snap recipe.
// Path: /~<owner>/+snap/<name>
func (c *Client) GetSnap(ctx context.Context, owner, name string) (Snap, error) {
	var s Snap
	path := fmt.Sprintf("/~%s/+snap/%s", owner, name)
	if err := c.GetJSON(ctx, path, &s); err != nil {
		return Snap{}, fmt.Errorf("fetching snap: %w", err)
	}
	return s, nil
}

// GetSnapByLink fetches a snap using its self_link.
func (c *Client) GetSnapByLink(ctx context.Context, selfLink string) (Snap, error) {
	var s Snap
	if err := c.GetJSON(ctx, selfLink, &s); err != nil {
		return Snap{}, fmt.Errorf("fetching snap: %w", err)
	}
	return s, nil
}

// RequestSnapBuilds requests builds for a snap package.
func (c *Client) RequestSnapBuilds(ctx context.Context, snapSelfLink, archiveLink, pocket string, channels map[string]string) (BuildRequest, error) {
	form := url.Values{
		"ws.op":   {"requestBuilds"},
		"archive": {archiveLink},
		"pocket":  {pocket},
	}
	if len(channels) > 0 {
		ch, _ := json.Marshal(channels)
		form.Set("channels", string(ch))
	}
	var br BuildRequest
	if err := c.PostJSON(ctx, snapSelfLink, form, &br); err != nil {
		return BuildRequest{}, fmt.Errorf("requesting snap builds: %w", err)
	}
	return br, nil
}

// GetSnapBuilds returns all builds for a snap.
func (c *Client) GetSnapBuilds(ctx context.Context, snapSelfLink string) ([]SnapBuild, error) {
	return GetAllPages[SnapBuild](ctx, c, snapSelfLink+"/builds")
}

// GetSnapBuild fetches a specific snap build.
func (c *Client) GetSnapBuild(ctx context.Context, selfLink string) (SnapBuild, error) {
	var b SnapBuild
	if err := c.GetJSON(ctx, selfLink, &b); err != nil {
		return SnapBuild{}, fmt.Errorf("fetching snap build: %w", err)
	}
	return b, nil
}

// RetrySnapBuild retries a failed snap build.
func (c *Client) RetrySnapBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"retry"}})
	return err
}

// CancelSnapBuild cancels a pending or in-progress snap build.
func (c *Client) CancelSnapBuild(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"cancel"}})
	return err
}

// ScheduleSnapBuildStoreUpload schedules a store upload for a snap build.
func (c *Client) ScheduleSnapBuildStoreUpload(ctx context.Context, buildSelfLink string) error {
	_, err := c.Post(ctx, buildSelfLink, url.Values{"ws.op": {"scheduleStoreUpload"}})
	return err
}

// GetSnapBuildFileURLs returns URLs for files produced by a snap build.
func (c *Client) GetSnapBuildFileURLs(ctx context.Context, buildSelfLink string) ([]string, error) {
	u := wsOpURL(buildSelfLink, "getFileUrls", nil)
	var urls []string
	if err := c.GetJSON(ctx, u, &urls); err != nil {
		return nil, fmt.Errorf("fetching build file URLs: %w", err)
	}
	return urls, nil
}

// DeleteRockRecipe deletes a rock recipe.
func (c *Client) DeleteRockRecipe(ctx context.Context, recipeSelfLink string) error {
	return c.Delete(ctx, recipeSelfLink)
}

// DeleteCharmRecipe deletes a charm recipe.
func (c *Client) DeleteCharmRecipe(ctx context.Context, recipeSelfLink string) error {
	return c.Delete(ctx, recipeSelfLink)
}

// DeleteSnap deletes a snap recipe.
func (c *Client) DeleteSnap(ctx context.Context, snapSelfLink string) error {
	return c.Delete(ctx, snapSelfLink)
}
