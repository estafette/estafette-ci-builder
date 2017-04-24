package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhenEvaluator(t *testing.T) {

	t.Run("ReturnsFalseIfInputIsEmpty", func(t *testing.T) {

		// act
		result, err := whenEvaluator("", make(map[string]interface{}, 0))

		assert.NotNil(t, err)
		assert.False(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		// act
		result, _ := whenEvaluator("3 > 2", make(map[string]interface{}, 0))

		assert.True(t, result)
	})

	t.Run("ReturnsTrueIfInputEvaluatesToTrueWithoutParameters", func(t *testing.T) {

		parameters := make(map[string]interface{}, 3)
		parameters["branch"] = "master"

		// act
		result, _ := whenEvaluator("branch == 'master'", parameters)

		assert.True(t, result)
	})

}

// parameters := make(map[string]interface{}, 3)
// parameters["branch"] = "master"
// parameters["trigger"] = "commit"
// parameters["status"] = "succeeded"

// AND
// &&

// OR
// ||

// NOT
// !

// EQUALS/IS
// ==

// IN
// in

// CONTAINS
// contains

// GREATHER THAN
// >

// LESS THAN
// <

// GREATHER THAN EQUALS
// >=

// LESS THAN EQUALS
// <=

// when:>
//   branch in [master, development] &&
//   trigger == commit

// when:>
//   branch == master &&
//   trigger != manual

// when:>
//   branch == master ||
//   (trigger == manual &&
//   status in [succeeded, failed])
