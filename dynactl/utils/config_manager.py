"""
Configuration management utility for dynactl
"""

import os
import json
import logging
import re
from typing import Any, Dict, Optional

logger = logging.getLogger("dynactl")


class ConfigManager:
    """Handles configuration file operations for dynactl"""
    
    # Define allowed configuration keys and their validation rules
    VALID_KEYS = {
        "cloud": {"type": str, "allowed_values": ["aws", "azure", "gcp", "on-prem"]},
        "registry.url": {"type": str},
        "registry.username": {"type": str},
        "registry.password": {"type": str},
        "cluster.context": {"type": str},
        "cluster.namespace": {"type": str},
        "namespace": {"type": str, "validator": lambda x: bool(re.match(r'^[a-z0-9]([-a-z0-9]*[a-z0-9])?$', x))},
    }
    
    def __init__(self, config_file: str):
        """Initialize with path to config file"""
        self.config_file = config_file
        self.config_data = {}
        self._load_config()
    
    def _load_config(self):
        """Load configuration from file"""
        if not os.path.exists(self.config_file):
            logger.debug(f"Config file not found, creating new config at {self.config_file}")
            # Create directory if it doesn't exist
            os.makedirs(os.path.dirname(self.config_file), exist_ok=True)
            self.config_data = {}
            self._save_config()
        else:
            try:
                with open(self.config_file, 'r') as f:
                    self.config_data = json.load(f)
                logger.debug(f"Loaded configuration from {self.config_file}")
            except json.JSONDecodeError:
                logger.warning(f"Invalid JSON in config file {self.config_file}, using empty config")
                self.config_data = {}
            except Exception as e:
                logger.error(f"Failed to load config file: {str(e)}")
                self.config_data = {}
    
    def _save_config(self):
        """Save configuration to file"""
        try:
            # Create directory if it doesn't exist
            os.makedirs(os.path.dirname(self.config_file), exist_ok=True)
            
            with open(self.config_file, 'w') as f:
                json.dump(self.config_data, f, indent=2)
            logger.debug(f"Saved configuration to {self.config_file}")
            return True
        except Exception as e:
            logger.error(f"Failed to save config file: {str(e)}")
            return False
    
    def get(self, key: str) -> Optional[Any]:
        """Get configuration value for a key"""
        if key not in self.VALID_KEYS:
            logger.error(f"Invalid key: {key}")
            return None
        
        return self.config_data.get(key)
    
    def set(self, key: str, value: Any) -> bool:
        """Set configuration value for a key"""
        # Validate key
        if key not in self.VALID_KEYS:
            logger.error(f"Invalid key: {key}")
            return False
        
        # Validate value type
        expected_type = self.VALID_KEYS[key]["type"]
        if not isinstance(value, expected_type):
            logger.error(f"Invalid value type for {key}: expected {expected_type.__name__}, got {type(value).__name__}")
            return False
        
        # Validate value against allowed values if specified
        if "allowed_values" in self.VALID_KEYS[key] and value not in self.VALID_KEYS[key]["allowed_values"]:
            allowed = self.VALID_KEYS[key]["allowed_values"]
            logger.error(f"Invalid value for {key}: must be one of {allowed}")
            return False
        
        # Run custom validator if specified
        if "validator" in self.VALID_KEYS[key]:
            validator = self.VALID_KEYS[key]["validator"]
            if not validator(value):
                logger.error(f"Invalid value for {key}: failed validation")
                if key == "namespace":
                    logger.error("Namespace must be a valid Kubernetes namespace name (lowercase alphanumeric characters or '-', must start and end with alphanumeric)")
                return False
        
        # Update config and save
        self.config_data[key] = value
        return self._save_config()
    
    def unset(self, key: str) -> bool:
        """Remove a key from the configuration"""
        if key not in self.VALID_KEYS:
            logger.error(f"Invalid key: {key}")
            return False
        
        if key in self.config_data:
            del self.config_data[key]
            return self._save_config()
        return True
    
    def get_all(self) -> Dict[str, Any]:
        """Get all configuration values"""
        return self.config_data.copy() 