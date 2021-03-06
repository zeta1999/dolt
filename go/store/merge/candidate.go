// Copyright 2019 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// This file incorporates work covered by the following copyright and
// permission notice:
//
// Copyright 2016 Attic Labs, Inc. All rights reserved.
// Licensed under the Apache License, version 2.0:
// http://www.apache.org/licenses/LICENSE-2.0

package merge

import (
	"context"

	"github.com/dolthub/dolt/go/store/d"
	"github.com/dolthub/dolt/go/store/types"
)

// candidate represents a collection that is a candidate to be merged. This
// interface exists to wrap Maps, Sets and Structs with a common API so that
// threeWayOrderedSequenceMerge() can remain agnostic to which kind of
// collections it's actually working with.
type candidate interface {
	diff(ctx context.Context, parent candidate, change chan<- types.ValueChanged) error
	get(ctx context.Context, k types.Value) (types.Value, bool, error)
	pathConcat(ctx context.Context, change types.ValueChanged, path types.Path) (out types.Path, err error)
	getValue() types.Value
}

type mapCandidate struct {
	m types.Map
}

func (mc mapCandidate) diff(ctx context.Context, p candidate, change chan<- types.ValueChanged) error {
	return mc.m.Diff(ctx, p.(mapCandidate).m, change)
}

func (mc mapCandidate) get(ctx context.Context, k types.Value) (types.Value, bool, error) {
	return mc.m.MaybeGet(ctx, k)
}

func (mc mapCandidate) pathConcat(ctx context.Context, change types.ValueChanged, path types.Path) (out types.Path, err error) {
	out = append(out, path...)
	if kind := change.Key.Kind(); kind == types.BoolKind || kind == types.StringKind || kind == types.FloatKind {
		out = append(out, types.NewIndexPath(change.Key))
	} else {
		h, err := change.Key.Hash(mc.m.Format())

		if err != nil {
			return nil, err
		}

		out = append(out, types.NewHashIndexPath(h))
	}
	return out, nil
}

func (mc mapCandidate) getValue() types.Value {
	return mc.m
}

type setCandidate struct {
	s types.Set
}

func (sc setCandidate) diff(ctx context.Context, p candidate, change chan<- types.ValueChanged) error {
	return sc.s.Diff(ctx, p.(setCandidate).s, change)
}

func (sc setCandidate) get(ctx context.Context, k types.Value) (types.Value, bool, error) {
	return k, true, nil
}

func (sc setCandidate) pathConcat(ctx context.Context, change types.ValueChanged, path types.Path) (out types.Path, err error) {
	out = append(out, path...)
	if kind := change.Key.Kind(); kind == types.BoolKind || kind == types.StringKind || kind == types.FloatKind {
		out = append(out, types.NewIndexPath(change.Key))
	} else {
		h, err := change.Key.Hash(sc.s.Format())

		if err != nil {
			return nil, err
		}

		out = append(out, types.NewHashIndexPath(h))
	}
	return out, nil
}

func (sc setCandidate) getValue() types.Value {
	return sc.s
}

type structCandidate struct {
	s types.Struct
}

func (sc structCandidate) diff(ctx context.Context, p candidate, change chan<- types.ValueChanged) error {
	return sc.s.Diff(ctx, p.(structCandidate).s, change)
}

func (sc structCandidate) get(ctx context.Context, key types.Value) (types.Value, bool, error) {
	if field, ok := key.(types.String); ok {
		return sc.s.MaybeGet(string(field))
	}

	return nil, false, nil
}

func (sc structCandidate) pathConcat(ctx context.Context, change types.ValueChanged, path types.Path) (out types.Path, err error) {
	out = append(out, path...)
	str, ok := change.Key.(types.String)
	if !ok {
		t, err := types.TypeOf(change.Key)

		var typeStr string
		if err == nil {
			typeStr, _ = t.Describe(ctx)
		}

		d.Panic("Field names must be strings, not %s", typeStr)
	}
	return append(out, types.NewFieldPath(string(str))), nil
}

func (sc structCandidate) getValue() types.Value {
	return sc.s
}
