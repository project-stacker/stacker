load helpers

function setup() {
    stacker_setup
}

function teardown() {
    cleanup
}

@test "import various sizes" {
  test_copy_buffer_size 512k tar
  test_copy_buffer_size 2m tar
  test_copy_buffer_size 512k tar.gz
  test_copy_buffer_size 2m tar.gz
}
