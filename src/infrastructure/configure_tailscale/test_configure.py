"""
Tests for Feature: Configure Tailscale
"""
import pulumi

# Mocking Pulumi Infrastructure
class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + '_id', args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}

pulumi.runtime.set_mocks(
    MyMocks(),
    preview=False
)

# Import the code under test AFTER setting mocks
from configure import create_auth_key

@pulumi.runtime.test
def test_create_key_default_values_success():
    """
    Test_CreateKey_DefaultValues_Success: Verify key creation with default parameters.
    """
    def check_defaults(args):
        # Verify inputs passed to the provider
        tags = args[0]
        ephemeral = args[1]
        reusable = args[2]
        
        assert tags == ["tag:mesh"]
        assert ephemeral is True
        assert reusable is True

    # Execute
    key = create_auth_key("test-key")
    
    # Verify using Output.all
    return pulumi.Output.all(key.tags, key.ephemeral, key.reusable).apply(check_defaults)

@pulumi.runtime.test
def test_create_key_custom_tags_success():
    """
    Test_CreateKey_CustomTags_Success: Verify key creation with custom tags.
    """
    custom_tags = ["tag:custom", "tag:dev"]
    
    def check_tags(tags):
        assert tags == custom_tags

    key = create_auth_key("custom-tag-key", tags=custom_tags)
    return key.tags.apply(check_tags)

@pulumi.runtime.test
def test_create_key_non_ephemeral_success():
    """
    Test_CreateKey_NonEphemeral_Success: Verify key creation when ephemeral is False.
    """
    def check_ephemeral(ephemeral):
        assert ephemeral is False

    key = create_auth_key("persistent-key", ephemeral=False)
    return key.ephemeral.apply(check_ephemeral)
