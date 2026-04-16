import os
import sys

# Add src/ to sys.path so `import mesh` works when running tests in-repo
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "src"))
